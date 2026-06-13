package ui

import (
	"context"
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
	minLeftWidth    = 18
	minFeedWidth    = 24
	statusBarLines  = 1
	inputBarLines   = 1
	feedHeaderLines = 2 // room title row + rule row
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

// Model is the entire console state. Per the project constraint, nothing lives
// in package globals — all mutable state is here.
type Model struct {
	cfg    config.Config
	client *natsclient.Client
	self   natsclient.Identity

	width, height      int
	leftW, feedW, midH int
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
		now:          time.Now(),
	}
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

// Update is the central event handler.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		return m.onResize(msg)

	case tea.KeyMsg:
		return m.onKey(msg)

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
	}
	return m, nil
}

// onResize recomputes the layout and resizes every component.
func (m Model) onResize(msg tea.WindowSizeMsg) (tea.Model, tea.Cmd) {
	m.width, m.height = msg.Width, msg.Height
	m.computeLayout()
	m.ready = true

	m.roomList.SetSize(m.leftW, m.roomListHeight())
	m.viewport.Width = m.feedW
	m.viewport.Height = m.midH - feedHeaderLines
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

// computeLayout derives panel dimensions from the terminal size.
func (m *Model) computeLayout() {
	m.leftW = m.width * 20 / 100
	if m.leftW < minLeftWidth {
		m.leftW = minLeftWidth
	}
	if m.leftW > m.width-minFeedWidth {
		m.leftW = m.width - minFeedWidth
	}
	if m.leftW < 1 {
		m.leftW = 1
	}
	// left panel carries a right border (+1 column).
	m.feedW = m.width - m.leftW - 1
	if m.feedW < 1 {
		m.feedW = 1
	}
	m.midH = m.height - statusBarLines - inputBarLines
	if m.midH < 1 {
		m.midH = 1
	}
}

// View composes the four regions.
func (m Model) View() string {
	if !m.ready {
		return "initializing nats-console…"
	}
	status := m.renderStatusBar()
	left := styleLeftPanel.Height(m.midH).Render(m.renderLeft())
	feed := m.renderFeedPanel()
	middle := lipgloss.JoinHorizontal(lipgloss.Top, left, feed)
	input := m.renderInput()
	return lipgloss.JoinVertical(lipgloss.Left, status, middle, input)
}

// renderLeft stacks the room list and presence panel within the left column.
func (m Model) renderLeft() string {
	rooms := m.renderRoomList()
	presence := m.renderPresence()
	return lipgloss.JoinVertical(lipgloss.Left, rooms, presence)
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
