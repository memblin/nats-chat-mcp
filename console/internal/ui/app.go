package ui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/memblin/nats-chat-mcp/console/internal/config"
	natsclient "github.com/memblin/nats-chat-mcp/console/internal/nats"
)

// directBucket is the reserved room-list key under which direct messages are
// collected. It is a UI bucket only — never published as a NATS room subject.
const directBucket = "direct"

// historyLimit is how many retained messages a room loads on first activation.
const historyLimit = 200

// Layout geometry.
const (
	minLeftWidth     = 18
	minFeedWidth     = 24
	statusBarLines   = 1
	helpBarLines     = 1
	inputBarLines    = 1
	feedHeaderLines  = 2 // room title row + rule row
	leftDividerLines = 2 // blank spacer + rule between room list and presence
)

// focusZone is which region currently receives navigation keys.
type focusZone int

const (
	zoneRooms focusZone = iota
	zoneFeed
	zoneCompose
)

// roomEntry is a row in the room list: its name and unread count.
type roomEntry struct {
	name   string
	unread int
}

// --- async messages fed back into Update ---

type tickMsg time.Time

type historyLoadedMsg struct {
	room string
	msgs []natsclient.Message
}

// evictedMsg reports the result of an eviction: how many participants were
// removed and, when the follow-up registry read succeeded, the fresh snapshot.
type evictedMsg struct {
	count     int
	agents    []natsclient.Presence
	refreshed bool
}

// confirmState backs the eviction confirmation modal. scope is the active room
// the cleanup targets, or "" for the bulk "everywhere" action; targets are the
// stale participants that will be evicted on confirm.
type confirmState struct {
	scope   string
	targets []natsclient.Presence
}

// Model is the entire console state. Per the project constraint, nothing lives
// in package globals — all mutable state is here.
type Model struct {
	cfg    config.Config
	client *natsclient.Client
	self   natsclient.Identity

	width, height      int
	leftW, feedW, midH int
	leftContentW       int
	feedContentW       int
	paneContentH       int
	ready, started     bool
	quitting           bool

	conn natsclient.ConnState

	rooms    []roomEntry
	active   int // index into rooms, -1 when none
	feeds    map[string][]natsclient.Message
	loaded   map[string]bool // history fetched for room
	presence []natsclient.Presence

	roomList list.Model
	viewport viewport.Model
	input    textinput.Model

	focus        focusZone
	newestBottom bool
	searching    bool
	searchQuery  string

	// mouseOn tracks whether mouse reporting is active. Turning it off (key "m")
	// hands mouse events back to the terminal so the operator can drag-select and
	// copy text from the feed; turning it on restores wheel-scroll and click.
	mouseOn bool

	// confirm is non-nil while an eviction confirmation modal is open.
	confirm *confirmState

	now time.Time
}

// New builds the initial model. Component sizes are set on the first
// WindowSizeMsg; here they start empty.
func New(cfg config.Config, client *natsclient.Client) Model {
	ti := textinput.New()
	ti.Prompt = "> "
	ti.Placeholder = "type a message…"
	ti.Focus()

	rl := list.New(nil, roomDelegate{}, 0, 0)
	rl.SetShowTitle(false)
	rl.SetShowStatusBar(false)
	rl.SetShowHelp(false)
	rl.SetShowPagination(false)
	rl.SetFilteringEnabled(false)
	rl.DisableQuitKeybindings()

	return Model{
		cfg:          cfg,
		client:       client,
		self:         client.Identity(),
		conn:         natsclient.Connected,
		active:       -1,
		feeds:        make(map[string][]natsclient.Message),
		loaded:       make(map[string]bool),
		roomList:     rl,
		viewport:     viewport.New(0, 0),
		input:        ti,
		focus:        zoneCompose,
		newestBottom: true,
		mouseOn:      true, // the program starts with WithMouseCellMotion
		now:          time.Now(),
	}
}

// toggleMouse flips mouse reporting and returns the command that applies it:
// disabling it lets the terminal handle selection/copy, enabling it restores
// wheel-scroll and click-to-focus.
func (m *Model) toggleMouse() tea.Cmd {
	m.mouseOn = !m.mouseOn
	if m.mouseOn {
		return tea.EnableMouseCellMotion
	}
	return tea.DisableMouse
}

// Init starts the periodic tick and the input cursor blink.
func (m Model) Init() tea.Cmd {
	return tea.Batch(tickCmd(), textinput.Blink)
}

// tickCmd schedules the 1-second clock used to refresh live idle times.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// historyCmd loads a room's recent history off the event loop.
func (m Model) historyCmd(room string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		msgs, err := client.History(context.Background(), room, historyLimit)
		if err != nil {
			msgs = nil
		}
		return historyLoadedMsg{room: room, msgs: msgs}
	}
}

// subscribeCmd joins a room's live stream (for unread counting) without loading
// history.
func (m Model) subscribeCmd(room string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_ = client.JoinRoom(context.Background(), room)
		return nil
	}
}

// sendCmd publishes a room message; the console receives its own message back
// via the subscription, so there is no local echo here.
func (m Model) sendCmd(room, content string) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		_, _ = client.SendRoom(context.Background(), room, content, "")
		return nil
	}
}

// evictCmd removes each target participant off the event loop, then re-reads the
// registry so the panel reflects the removals without waiting for the next poll.
func (m Model) evictCmd(targets []natsclient.Presence) tea.Cmd {
	client := m.client
	return func() tea.Msg {
		count := 0
		for _, p := range targets {
			if err := client.Evict(context.Background(), p); err == nil {
				count++
			}
		}
		agents, err := client.ListPresence(context.Background())
		return evictedMsg{count: count, agents: agents, refreshed: err == nil}
	}
}

// Update is the central event handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)

	case tea.KeyMsg:
		return m.onKey(msg)

	case tea.MouseMsg:
		return m.onMouse(msg)

	case tickMsg:
		m.now = time.Time(msg)
		return m, tickCmd()

	case natsclient.ConnEvent:
		m.conn = msg.State
		return m, nil

	case natsclient.PresenceEvent:
		m.presence = msg.Agents
		return m.onPresence()

	case natsclient.MessageEvent:
		return m.onMessage(msg)

	case historyLoadedMsg:
		m.feeds[msg.room] = msg.msgs
		m.loaded[msg.room] = true
		if m.activeName() == msg.room {
			m.refreshViewport()
		}
		return m, nil

	case evictedMsg:
		if msg.refreshed {
			m.presence = msg.agents
		}
		return m, nil
	}
	return m, nil
}

// onResize recomputes the layout and resizes every component.
func (m Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	m.computeLayout()
	m.ready = true

	m.roomList.SetSize(m.leftContentW, m.roomListHeight())
	m.viewport.Width = m.feedContentW
	m.viewport.Height = m.paneContentH - feedHeaderLines
	if m.viewport.Height < 1 {
		m.viewport.Height = 1
	}
	m.input.Width = m.width - 4

	var cmd tea.Cmd
	if !m.started {
		m.started = true
		if m.cfg.DefaultRoom != "" {
			cmd = m.ensureAndActivate(m.cfg.DefaultRoom)
		}
	}
	m.refreshViewport()
	return m, cmd
}

// computeLayout derives panel dimensions from the terminal size. Each pane is a
// bordered box (+2 in each axis); the vertical stack is status bar + middle +
// help line + input line.
func (m *Model) computeLayout() {
	m.leftW = m.width * 22 / 100
	if m.leftW < minLeftWidth {
		m.leftW = minLeftWidth
	}
	if m.leftW > m.width-minFeedWidth {
		m.leftW = m.width - minFeedWidth
	}
	if m.leftW < 3 {
		m.leftW = 3
	}
	m.feedW = m.width - m.leftW
	if m.feedW < 3 {
		m.feedW = 3
	}
	m.leftContentW = m.leftW - 2 // rounded border on both sides
	m.feedContentW = m.feedW - 2
	if m.leftContentW < 1 {
		m.leftContentW = 1
	}
	if m.feedContentW < 1 {
		m.feedContentW = 1
	}

	m.midH = m.height - statusBarLines - helpBarLines - inputBarLines
	if m.midH < 3 {
		m.midH = 3
	}
	m.paneContentH = m.midH - 2 // top/bottom border
	if m.paneContentH < 1 {
		m.paneContentH = 1
	}
}

// View composes the five regions: status bar, the two panes, a help line, and
// the input bar.
func (m Model) View() string {
	if !m.ready {
		return "initializing nats-chat-console…"
	}
	status := m.renderStatusBar()
	middle := lipgloss.JoinHorizontal(lipgloss.Top, m.renderLeftPane(), m.renderFeedPane())
	if m.confirm != nil {
		// A modal owns the middle region while a destructive action awaits y/N.
		middle = m.renderConfirmModal()
	}
	help := m.renderHelp()
	input := m.renderInput()
	return lipgloss.JoinVertical(lipgloss.Left, status, middle, help, input)
}

// renderConfirmModal draws the eviction confirmation as a centered box filling
// the middle region (so the overall layout height is unchanged).
func (m Model) renderConfirmModal() string {
	c := m.confirm
	scope := "everywhere"
	if c.scope != "" {
		scope = "\"" + c.scope + "\""
	}

	var b strings.Builder
	if len(c.targets) == 0 {
		b.WriteString(styleModalTitle.Render("No stale participants"))
		b.WriteString("\n\n")
		fmt.Fprintf(&b, "Nothing to evict %s.", scope)
		b.WriteString("\n\n")
		b.WriteString(styleModalDim.Render("any key  close"))
	} else {
		b.WriteString(styleModalTitle.Render(
			fmt.Sprintf("Evict %d stale participant(s) %s", len(c.targets), scope)))
		b.WriteString("\n\n")
		const maxShown = 8
		for i, p := range c.targets {
			if i >= maxShown {
				fmt.Fprintf(&b, "  … and %d more\n", len(c.targets)-maxShown)
				break
			}
			idle := natsclient.FormatIdle(m.now.Sub(p.LastSeenTime()))
			fmt.Fprintf(&b, "  • %-16s idle %s\n", truncate(p.Name, 16), idle)
		}
		b.WriteString("\n")
		b.WriteString(styleModalDim.Render(
			"Removes their presence record and delivery consumers.\n" +
				"No messages are deleted. A still-live agent re-registers\n" +
				"on its next activity."))
		b.WriteString("\n\n")
		b.WriteString(styleModalKey.Render("y") + styleModalDim.Render("  evict") +
			"      " + styleModalKey.Render("any key") + styleModalDim.Render("  cancel"))
	}

	box := styleModalBox.Render(b.String())
	return lipgloss.Place(m.width, m.midH, lipgloss.Center, lipgloss.Center, box)
}

// renderLeftPane draws the bordered left column (room list + presence),
// separated by a divider; its border is accented when the rooms zone has focus.
func (m Model) renderLeftPane() string {
	content := lipgloss.JoinVertical(lipgloss.Left,
		m.renderRoomList(),
		m.renderLeftDivider(),
		m.renderPresence(),
	)
	return paneStyle(m.focus == zoneRooms).
		Width(m.leftContentW).
		Height(m.paneContentH).
		Render(content)
}

// renderLeftDivider is the spacer + horizontal rule (leftDividerLines tall) that
// separates the room list from the presence panel.
func (m Model) renderLeftDivider() string {
	rule := styleSectionDivider.Render(strings.Repeat("─", m.leftContentW))
	return lipgloss.JoinVertical(lipgloss.Left, "", rule)
}

// --- helpers shared across files ---

// activeName returns the name of the active room, or "" if none.
func (m Model) activeName() string {
	if m.active < 0 || m.active >= len(m.rooms) {
		return ""
	}
	return m.rooms[m.active].name
}

// roomIndex finds a room entry by name, or -1.
func (m Model) roomIndex(name string) int {
	for i, r := range m.rooms {
		if r.name == name {
			return i
		}
	}
	return -1
}

// ensureRoom adds a room to the list if absent, returning a subscribe command
// for real (non-direct) rooms so unread counting works before activation.
func (m *Model) ensureRoom(name string) tea.Cmd {
	if m.roomIndex(name) >= 0 {
		return nil
	}
	m.rooms = append(m.rooms, roomEntry{name: name})
	m.syncRoomItems()
	if name == directBucket {
		return nil
	}
	return m.subscribeCmd(name)
}

// ensureAndActivate adds (if needed) and activates a room, batching any
// resulting subscribe/history commands.
func (m *Model) ensureAndActivate(name string) tea.Cmd {
	sub := m.ensureRoom(name)
	act := m.activate(m.roomIndex(name))
	return tea.Batch(sub, act)
}

// activate switches the active room, clears its unread badge, and loads history
// the first time it is opened.
func (m *Model) activate(i int) tea.Cmd {
	if i < 0 || i >= len(m.rooms) {
		return nil
	}
	m.active = i
	m.rooms[i].unread = 0
	m.syncRoomItems()
	m.roomList.Select(i)

	var cmd tea.Cmd
	name := m.rooms[i].name
	if !m.loaded[name] && name != directBucket {
		cmd = m.historyCmd(name)
	}
	m.refreshViewport()
	return cmd
}

// nextRoom / prevRoom move the active selection with wrap-around.
func (m *Model) nextRoom() tea.Cmd {
	if len(m.rooms) == 0 {
		return nil
	}
	return m.activate((m.active + 1 + len(m.rooms)) % len(m.rooms))
}

func (m *Model) prevRoom() tea.Cmd {
	if len(m.rooms) == 0 {
		return nil
	}
	return m.activate((m.active - 1 + len(m.rooms)) % len(m.rooms))
}

// onMessage records an incoming message, updating unread or the live feed.
func (m Model) onMessage(ev natsclient.MessageEvent) (tea.Model, tea.Cmd) {
	bucket := ev.Room
	if bucket == "" {
		bucket = directBucket
	}
	cmd := m.ensureRoom(bucket)
	m.feeds[bucket] = append(m.feeds[bucket], ev.Msg)
	if bucket == m.activeName() {
		m.refreshViewport()
	} else if i := m.roomIndex(bucket); i >= 0 {
		m.rooms[i].unread++
		m.syncRoomItems()
	}
	return m, cmd
}

// onPresence discovers rooms peers are in and adds them to the list so their
// traffic is tracked.
func (m Model) onPresence() (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	for _, p := range m.presence {
		for _, r := range p.Rooms {
			if r != "" {
				if c := m.ensureRoom(r); c != nil {
					cmds = append(cmds, c)
				}
			}
		}
	}
	return m, tea.Batch(cmds...)
}

// refreshViewport re-renders the active room's feed with the current sort and
// search applied, pinning to the bottom when showing newest-at-bottom.
func (m *Model) refreshViewport() {
	name := m.activeName()
	msgs := m.feeds[name]
	msgs = filterMessages(msgs, m.searchQuery)
	msgs = sortMessages(msgs, m.newestBottom)
	m.viewport.SetContent(renderFeed(msgs, m.feedW, m.self.ID))
	if m.newestBottom && !m.searching {
		m.viewport.GotoBottom()
	}
}
