package nats

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ---------------------------------------------------------------------------
// Wire conventions — ported verbatim from src/stream-manager.ts. Every subject,
// stream, consumer and bucket name the console touches is defined here as a
// constant or builder; no NATS string literals live anywhere else in the tree.
// ---------------------------------------------------------------------------

const (
	// StreamRooms / StreamDirect are the two JetStream streams the MCP creates.
	StreamRooms  = "CLAUDE_CHAT_ROOMS"
	StreamDirect = "CLAUDE_CHAT_DIRECT"

	// PresenceKV is the JetStream KV bucket the MCP uses as its live agent
	// registry — what list_agents reads. The console writes its own record here
	// so a human operator shows up alongside agents.
	PresenceKV = "claude_chat_agents"

	// Stream subject spaces.
	roomSubjectWildcard   = "chat.room.>"
	directSubjectWildcard = "chat.direct.>"

	// Subject token fragments. RoomSubject/DirectSubject assemble the full
	// subject; nothing else should concatenate these by hand.
	roomSubjectPrefix   = "chat.room."
	directSubjectPrefix = "chat.direct."
	subjectMsgSuffix    = ".msg"

	// Stream retention — must match the MCP so a console-created stream (when
	// the console runs before any MCP has) is identical to an MCP-created one.
	messageMaxAge          = 24 * time.Hour
	messageMaxMsgPerSubj   = 1000
	consumerInactiveThresh = 7 * 24 * time.Hour

	// Presence bucket config. The TTL is the linger window: the server expires a
	// record this long after its last write, so a session that dies without
	// publishing a departure (DeletePresence) clears on its own. Kept short
	// because every live participant refreshes well inside it — the console
	// heartbeats every 30s, and an agent re-syncs on each tool call and at the
	// top of every wait_for_message (≤2min in a wait loop). Must match the MCP's
	// PRESENCE_TTL_MS (src/stream-manager.ts) so a console-created bucket is
	// identical to an MCP-created one.
	presenceHistory = 1
	presenceTTL     = 15 * time.Minute
)

// RoomSubject returns the subject room broadcasts are published to.
func RoomSubject(room string) string { return roomSubjectPrefix + room + subjectMsgSuffix }

// DirectSubject returns the subject direct messages to an identity land on.
func DirectSubject(id string) string { return directSubjectPrefix + id + subjectMsgSuffix }

// roomConsumerName / directConsumerName reproduce the MCP's durable-name scheme
// so the console gets its OWN delivery cursor (keyed by its own identity id) and
// never shares or disturbs an agent's consumer.
func roomConsumerName(id, room string) string { return "room_" + id + "_" + room }
func directConsumerName(id string) string     { return "direct_" + id }

// ---------------------------------------------------------------------------
// Identity
// ---------------------------------------------------------------------------

// Identity is the console operator's handle on the bus. ID is a subject/durable
// safe token (like the MCP's "a"+hex agent ids, but "h"+hex to denote a human);
// Name is the display name configured by the operator.
type Identity struct {
	ID   string
	Name string
}

// NewIdentity mints a fresh human identity for this console session. The id is
// random per run so two consoles using the same display name never collide on a
// durable consumer; the stale presence record TTLs out on its own.
func NewIdentity(name string) Identity {
	var b [15]byte
	if _, err := rand.Read(b[:]); err != nil {
		return Identity{ID: fmt.Sprintf("h%d", time.Now().UnixNano()), Name: name}
	}
	return Identity{ID: "h" + hex.EncodeToString(b[:]), Name: name}
}

// ---------------------------------------------------------------------------
// Connection events — emitted to the UI via a sink so this package never
// imports bubbletea. tea.Msg is `any`, so main can wire EventSink straight to
// program.Send.
// ---------------------------------------------------------------------------

// ConnState is the connection lifecycle the status bar reflects.
type ConnState int

const (
	// Disconnected means no usable connection yet (or a terminal failure).
	Disconnected ConnState = iota
	// Connected means the link is live.
	Connected
	// Reconnecting means nats.go is retrying with backoff.
	Reconnecting
)

// MessageEvent is emitted for every message delivered on a subscribed room or
// the console's direct inbox.
type MessageEvent struct {
	Room string // empty for a direct message
	Msg  Message
}

// PresenceEvent carries the latest snapshot of the presence registry.
type PresenceEvent struct {
	Agents []Presence
}

// ConnEvent reports a connection-state transition.
type ConnEvent struct {
	State ConnState
}

// EventSink receives asynchronously-produced events (messages, presence,
// connection changes). It is safe to call from any goroutine.
type EventSink func(ev any)

// ---------------------------------------------------------------------------
// Client
// ---------------------------------------------------------------------------

// Client owns the NATS connection, JetStream handle, presence bucket, and the
// set of live room/direct subscriptions. It is the only thing in the program
// that talks to NATS; the UI model calls into it and receives events via sink.
type Client struct {
	nc   *nats.Conn
	js   jetstream.JetStream
	kv   jetstream.KeyValue
	id   Identity
	sink EventSink

	mu     sync.Mutex
	subs   map[string]jetstream.ConsumeContext // room -> live consumer
	direct jetstream.ConsumeContext            // direct-inbox consumer
	rooms  map[string]struct{}                 // rooms currently joined (for presence)
}

// Identity exposes the operator identity (read-only) for the UI/status bar.
func (c *Client) Identity() Identity { return c.id }

// Connect dials NATS, binds JetStream, ensures the shared infrastructure exists,
// and creates the console's direct inbox. Connection-state transitions are
// forwarded to sink with exponential reconnect backoff; nats.go never gives up.
func Connect(ctx context.Context, url string, id Identity, sink EventSink) (*Client, error) {
	c := &Client{
		id:    id,
		sink:  sink,
		subs:  make(map[string]jetstream.ConsumeContext),
		rooms: make(map[string]struct{}),
	}

	nc, err := nats.Connect(url,
		nats.Name("nats-chat-console:"+id.Name),
		nats.MaxReconnects(-1),
		// Exponential backoff, capped, so a downed server is retried forever
		// without hammering it.
		nats.CustomReconnectDelay(func(attempts int) time.Duration {
			d := time.Duration(attempts) * 250 * time.Millisecond
			if d > 5*time.Second {
				d = 5 * time.Second
			}
			return d
		}),
		nats.DisconnectErrHandler(func(_ *nats.Conn, _ error) { c.emit(ConnEvent{State: Reconnecting}) }),
		nats.ReconnectHandler(func(_ *nats.Conn) { c.emit(ConnEvent{State: Connected}) }),
		nats.ClosedHandler(func(_ *nats.Conn) { c.emit(ConnEvent{State: Disconnected}) }),
	)
	if err != nil {
		return nil, fmt.Errorf("connect to NATS at %s: %w", url, err)
	}
	c.nc = nc

	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("init JetStream: %w", err)
	}
	c.js = js

	if err := c.ensureInfrastructure(ctx); err != nil {
		nc.Close()
		return nil, err
	}

	if err := c.subscribeDirect(ctx); err != nil {
		nc.Close()
		return nil, err
	}

	c.emit(ConnEvent{State: Connected})
	return c, nil
}

// ensureInfrastructure creates the streams and presence bucket if absent, with
// config identical to the MCP's ensureInfrastructure. Idempotent.
func (c *Client) ensureInfrastructure(ctx context.Context) error {
	if _, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:              StreamRooms,
		Subjects:          []string{roomSubjectWildcard},
		Retention:         jetstream.LimitsPolicy,
		Storage:           jetstream.FileStorage,
		Discard:           jetstream.DiscardOld,
		MaxAge:            messageMaxAge,
		MaxMsgsPerSubject: messageMaxMsgPerSubj,
	}); err != nil {
		return fmt.Errorf("ensure rooms stream: %w", err)
	}
	if _, err := c.js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:              StreamDirect,
		Subjects:          []string{directSubjectWildcard},
		Retention:         jetstream.LimitsPolicy,
		Storage:           jetstream.FileStorage,
		Discard:           jetstream.DiscardOld,
		MaxAge:            messageMaxAge,
		MaxMsgsPerSubject: messageMaxMsgPerSubj,
	}); err != nil {
		return fmt.Errorf("ensure direct stream: %w", err)
	}

	kv, err := c.js.CreateOrUpdateKeyValue(ctx, jetstream.KeyValueConfig{
		Bucket:  PresenceKV,
		History: presenceHistory,
		TTL:     presenceTTL,
	})
	if err != nil {
		return fmt.Errorf("ensure presence bucket: %w", err)
	}
	c.kv = kv
	return nil
}

// emit forwards an event to the sink if one is registered.
func (c *Client) emit(ev any) {
	if c.sink != nil {
		c.sink(ev)
	}
}

// ---------------------------------------------------------------------------
// Publishing
// ---------------------------------------------------------------------------

// SendRoom publishes a room broadcast as this identity and returns the message
// that was sent (so the UI can echo it immediately).
func (c *Client) SendRoom(ctx context.Context, room, content, replyTo string) (Message, error) {
	m := newMessage(c.id, content, room, replyTo)
	data, err := m.encode()
	if err != nil {
		return Message{}, err
	}
	if _, err := c.js.Publish(ctx, RoomSubject(room), data); err != nil {
		return Message{}, fmt.Errorf("publish to %s: %w", room, err)
	}
	return m, nil
}

// SendDirect publishes a direct message to a target identity id.
func (c *Client) SendDirect(ctx context.Context, toID, content, replyTo string) (Message, error) {
	m := newMessage(c.id, content, "", replyTo)
	data, err := m.encode()
	if err != nil {
		return Message{}, err
	}
	if _, err := c.js.Publish(ctx, DirectSubject(toID), data); err != nil {
		return Message{}, fmt.Errorf("publish direct: %w", err)
	}
	return m, nil
}

// ---------------------------------------------------------------------------
// History — bounded replay over a room's retained messages (oldest first).
// ---------------------------------------------------------------------------

// History fetches up to limit of the most recent retained messages for a room
// using an ephemeral ordered consumer, exactly like the MCP's get_history.
func (c *Client) History(ctx context.Context, room string, limit int) ([]Message, error) {
	cons, err := c.js.OrderedConsumer(ctx, StreamRooms, jetstream.OrderedConsumerConfig{
		FilterSubjects: []string{RoomSubject(room)},
	})
	if err != nil {
		return nil, fmt.Errorf("history consumer for %s: %w", room, err)
	}

	batch, err := cons.Fetch(messageMaxMsgPerSubj, jetstream.FetchMaxWait(1500*time.Millisecond))
	if err != nil {
		return nil, fmt.Errorf("history fetch for %s: %w", room, err)
	}

	out := make([]Message, 0, limit)
	for m := range batch.Messages() {
		msg, perr := ParseMessage(m.Data())
		if perr != nil {
			continue // skip malformed payload, like the MCP drain loop
		}
		out = append(out, msg)
	}
	if err := batch.Error(); err != nil {
		return nil, fmt.Errorf("history stream for %s: %w", room, err)
	}

	if limit > 0 && len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// ---------------------------------------------------------------------------
// Live subscriptions — one durable push consumer per joined room, plus one for
// the direct inbox. Deliveries are parsed, acked, and emitted to the sink.
// ---------------------------------------------------------------------------

// JoinRoom registers the room for presence, creates (or rebinds) the console's
// durable consumer for it, and starts streaming deliveries to the sink. Safe to
// call repeatedly for the same room.
func (c *Client) JoinRoom(ctx context.Context, room string) error {
	c.mu.Lock()
	if _, ok := c.subs[room]; ok {
		c.mu.Unlock()
		return nil
	}
	c.mu.Unlock()

	cons, err := c.js.CreateOrUpdateConsumer(ctx, StreamRooms, jetstream.ConsumerConfig{
		Durable:           roomConsumerName(c.id.ID, room),
		AckPolicy:         jetstream.AckExplicitPolicy,
		DeliverPolicy:     jetstream.DeliverNewPolicy,
		FilterSubject:     RoomSubject(room),
		InactiveThreshold: consumerInactiveThresh,
	})
	if err != nil {
		return fmt.Errorf("room consumer for %s: %w", room, err)
	}

	cc, err := cons.Consume(func(m jetstream.Msg) {
		if msg, perr := ParseMessage(m.Data()); perr == nil {
			c.emit(MessageEvent{Room: room, Msg: msg})
		}
		_ = m.Ack()
	})
	if err != nil {
		return fmt.Errorf("consume room %s: %w", room, err)
	}

	c.mu.Lock()
	c.subs[room] = cc
	c.rooms[room] = struct{}{}
	c.mu.Unlock()

	return c.PublishPresence(ctx)
}

// LeaveRoom stops the room's consumer and drops it from presence. The durable
// itself is left to expire (inactive_threshold), matching the MCP, so a brief
// rejoin keeps its cursor.
func (c *Client) LeaveRoom(ctx context.Context, room string) error {
	c.mu.Lock()
	if cc, ok := c.subs[room]; ok {
		cc.Stop()
		delete(c.subs, room)
	}
	delete(c.rooms, room)
	c.mu.Unlock()
	return c.PublishPresence(ctx)
}

// subscribeDirect starts the console's direct-inbox consumer so DMs addressed
// to this identity are delivered in real time.
func (c *Client) subscribeDirect(ctx context.Context) error {
	cons, err := c.js.CreateOrUpdateConsumer(ctx, StreamDirect, jetstream.ConsumerConfig{
		Durable:           directConsumerName(c.id.ID),
		AckPolicy:         jetstream.AckExplicitPolicy,
		DeliverPolicy:     jetstream.DeliverNewPolicy,
		FilterSubject:     DirectSubject(c.id.ID),
		InactiveThreshold: consumerInactiveThresh,
	})
	if err != nil {
		return fmt.Errorf("direct consumer: %w", err)
	}
	cc, err := cons.Consume(func(m jetstream.Msg) {
		if msg, perr := ParseMessage(m.Data()); perr == nil {
			c.emit(MessageEvent{Room: "", Msg: msg})
		}
		_ = m.Ack()
	})
	if err != nil {
		return fmt.Errorf("consume direct: %w", err)
	}
	c.mu.Lock()
	c.direct = cc
	c.mu.Unlock()
	return nil
}

// JoinedRooms returns the rooms currently subscribed, sorted by the caller's
// preference elsewhere; order here is unspecified.
func (c *Client) JoinedRooms() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]string, 0, len(c.rooms))
	for r := range c.rooms {
		out = append(out, r)
	}
	return out
}

// Close departs all rooms (publishing a final presence delete) and tears down
// the connection.
func (c *Client) Close(ctx context.Context) {
	c.mu.Lock()
	for _, cc := range c.subs {
		cc.Stop()
	}
	if c.direct != nil {
		c.direct.Stop()
	}
	c.subs = make(map[string]jetstream.ConsumeContext)
	c.rooms = make(map[string]struct{})
	c.mu.Unlock()

	// Best-effort departure so peers see us leave promptly.
	_ = c.DeletePresence(ctx)
	if c.nc != nil {
		c.nc.Close()
	}
}
