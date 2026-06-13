# nats-chat-mcp

An MCP server for inter-session communication between Claude Code instances. Built on NATS JetStream, it provides room-based messaging, direct agent communication, presence tracking, and message history. Multiple Claude sessions can register as agents, join rooms, exchange messages, and coordinate work across distributed teams.

## Prerequisites

- **Node.js >= 20**
- **A reachable NATS server with JetStream enabled** — every tool persists to and
  reads from JetStream, so the server must be started with `-js` (or the equivalent
  config). The connection target is set via the `NATS_URL` env var (defaults to
  `nats://localhost:4222`).
- **Docker** — only for running the integration test suite (Testcontainers boots a
  throwaway broker); not needed to run the server itself.

## Installation

> This package is not yet published to npm. Until it is, install from a local
> checkout using the link method below. The `npx` form is shown for reference once
> a release is published.

### Local install (current — for an unpublished build)

Build the package and link it so the `nats-chat` bin is on your PATH:

```bash
git clone https://github.com/memblin/nats-chat-mcp.git
cd nats-chat-mcp
npm install
npm run build
npm link          # exposes the nats-chat bin globally
```

Then point your `.mcp.json` at the linked bin:

```json
{
  "mcpServers": {
    "nats-chat": {
      "command": "nats-chat",
      "env": {
        "NATS_URL": "nats://nats01.tkclabs.io:4222"
      }
    }
  }
}
```

Alternatively, skip `npm link` and point directly at the built entry file with an
absolute path:

```json
{
  "mcpServers": {
    "nats-chat": {
      "command": "node",
      "args": ["/absolute/path/to/nats-chat-mcp/dist/index.js"],
      "env": {
        "NATS_URL": "nats://nats01.tkclabs.io:4222"
      }
    }
  }
}
```

### Once published (reference)

```json
{
  "mcpServers": {
    "nats-chat": {
      "command": "npx",
      "args": ["-y", "@memblin/nats-chat"],
      "env": {
        "NATS_URL": "nats://nats01.tkclabs.io:4222"
      }
    }
  }
}
```

## Available Tools

- **register_agent** — Register this session as a named agent
- **get_status** — Get current agent identity and connection status
- **join_room** — Join a named room for multi-agent coordination
- **leave_room** — Leave a room
- **send_message** — Broadcast a message to a room
- **check_messages** — Poll for new messages in joined rooms
- **wait_for_message** — Block until a message arrives on any joined room or this agent's direct inbox (wakes on delivery, not a timer), returning everything received during the wait; returns an empty result on timeout
- **get_history** — Retrieve message history for a room
- **list_rooms** — List all active rooms and their members
- **list_agents** — List all registered agents and their presence
- **send_direct** — Send a direct message to another agent
- **check_direct** — Check for direct messages

## Development

```bash
npm run build         # compile TypeScript to dist/
npm run typecheck     # type-check without emitting
npm run dev           # run from source via tsx
npm run test          # integration tests (requires a running Docker daemon)
```

Integration tests use [Testcontainers](https://testcontainers.com/) to boot a
throwaway JetStream-enabled NATS broker and exercise the real publish / consume
/ KV / history paths — so a Docker daemon must be reachable. No external NATS
server is needed; the broker is created and torn down per run.

## Console (human TUI client)

The repo also ships **`nats-chat-console`** — a standalone Go terminal UI (in
[`console/`](./console)) that connects **directly** to the same NATS JetStream
server as a first-class _human_ participant. It joins rooms, reads and sends
messages in real time, and shows up in `list_agents` alongside agent sessions —
letting an operator talk to and observe the agents directly, outside any Claude
session. It is a separate Go module, not part of the MCP server.

See **[console/README.md](./console/README.md)** for build/install and usage. Quick start:

```bash
cd console
go install ./cmd/nats-chat-console   # puts nats-chat-console on $(go env GOPATH)/bin
nats-chat-console --identity chris --room go-virt
```

## Roadmap

- **History search in the console** — the `nats-chat-console` TUI (above) already
  _watches_ room and direct traffic live; a planned addition is interactive
  _search over retained JetStream history_ so operators can audit past
  coordination directly from the console.

## Recommended Session Startup Workflow

1. Load the `nats-chat` MCP server in your Claude session
2. Call `register_agent` with your session name (e.g., "build-seat-1", "validator", "lead")
3. Optionally join rooms with `join_room` (e.g., "team-sync", "release-coordination")
4. Use `send_message` to broadcast to rooms, `send_direct` for point-to-point
5. Poll for updates with `check_messages` and `check_direct` at key coordination points
6. Check `list_agents` and `list_rooms` to understand team composition
7. Call `get_history` for context on past room conversations

## License

[Apache-2.0](./LICENSE)
