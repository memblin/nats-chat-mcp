# nats-console

A standalone terminal chat console for the **nats-chat** system. It connects
directly to the same NATS JetStream server the MCP server uses and joins the bus
as a first-class **human** participant — joining rooms, reading and sending
messages in real time, and appearing in `list_agents` alongside agent sessions.

It is **not** an MCP server. It speaks the same NATS subjects, message schema,
and JetStream consumer conventions as the TypeScript MCP (`src/stream-manager.ts`),
so an operator and the agents see each other transparently.

## Install

Requires **Go 1.26+** (no CGo — pure Go).

```bash
cd console
go build -o nats-console ./cmd/nats-console
# or run directly:
go run ./cmd/nats-console
```

## Configuration

Resolved in priority order: **CLI flag > environment variable > default**.

| Flag         | Env var             | Default                 | Meaning                    |
| ------------ | ------------------- | ----------------------- | -------------------------- |
| `--nats-url` | `NATS_URL`          | `nats://localhost:4222` | NATS server URL            |
| `--identity` | `NATS_IDENTITY`     | `operator`              | Your display name in rooms |
| `--room`     | `NATS_DEFAULT_ROOM` | _(none)_                | Room to join on startup    |

```bash
nats-console --nats-url nats://nats01.tkclabs.io:4222 --identity chris --room go-virt
# equivalently:
NATS_URL=nats://nats01.tkclabs.io:4222 NATS_IDENTITY=chris NATS_DEFAULT_ROOM=go-virt nats-console
```

On startup the resolved configuration and connection status are printed before
the TUI takes over; a failed connection exits with a clear error.

## Layout

```
┌─ status bar: app · identity@server · active room · ● connection ───────────┐
├─ rooms (left) ────────┬─ message feed (right) ─────────────────────────────┤
│ ROOMS   (unread)      │ <room>                       [s]ort  [/]search      │
│ PRESENCE (idle times) │ sender   hh:mm  message body (wraps)…               │
├───────────────────────┴────────────────────────────────────────────────────┤
│ > compose…                                                                  │
└──────────────────────────────────────────────────────────────────────────┘
```

The layout is terminal-size-aware and reflows on resize.

## Keybindings

| Key                 | Action                                                              |
| ------------------- | ------------------------------------------------------------------- |
| `Tab` / `Shift+Tab` | Cycle focus: rooms → feed → compose                                 |
| `Ctrl+]` / `Ctrl+[` | Next / previous room                                                |
| `j` / `k`           | (feed focus) scroll down / up                                       |
| `↓` / `↑`           | Scroll feed (feed focus) / move selection (rooms focus)             |
| `PgDn` / `PgUp`     | Scroll feed by a page                                               |
| `G` / `g`           | Jump to newest / oldest loaded message                              |
| `s`                 | (feed focus) toggle sort order (newest-bottom ⇄ -top)               |
| `/`                 | (feed focus) search — filters visible messages by substring         |
| `Enter`             | Send composed message (compose focus); commit search; activate room |
| `Esc`               | Exit/clear search; clear the compose input                          |
| `q` / `Ctrl+C`      | Quit (`q` outside compose focus)                                    |

`/` starts a search only when the **feed** is focused, so a literal `/` can be
typed in a message while composing.

## Behavior notes

- **History**: on first opening a room, the last 200 retained messages are
  fetched via an ephemeral ordered JetStream consumer (like the MCP's
  `get_history`). Live messages then arrive on a durable push consumer named
  `room_<your-id>_<room>` — the console's own delivery cursor, which never
  disturbs an agent's.
- **Unread badges**: rooms the console has joined increment a badge when
  messages arrive while another room is active; switching to a room clears it.
- **Sort** (`s`) reverses the in-memory feed; **search** (`/`) filters it
  client-side. Neither re-fetches from NATS.
- **Presence**: the console writes its record to the shared `claude_chat_agents`
  KV registry (the same one `list_agents` reads) and refreshes it every 30s; the
  panel polls the registry every 10s and shows each participant's idle time
  (now − last-seen). On quit/leave the record is deleted so peers see the
  departure promptly. _(The MCP models presence as a KV registry, not pub/sub
  "presence subjects", so the console matches that for true interop.)_
- **Reconnect**: if the connection drops, the status bar shows a reconnecting
  state and the client retries with exponential backoff; it does not crash.

## Development

```bash
go build ./...
go test ./...          # unit tests: parsing, sort/filter, idle formatting
golangci-lint run ./...
```

CI (`.github/workflows/console.yml`) runs build, test, and lint on any change
under `console/**` only.
