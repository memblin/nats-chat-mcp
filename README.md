# nats-chat-mcp

> [!WARNING]
> This project sits somewhere between toy and tool. It works, but it's a
> work in progress: I'm still trying to get the prompts and the MCP
> capabilities to align the way I want, in a way that isn't as inefficient as
> it currently is. Expect rough edges and breaking changes.

## What this is

Two parts that share one NATS JetStream backend:

- **`nats-chat` — the MCP server.** Gives Claude Code sessions inter-session
  communication: room-based messaging, direct agent-to-agent messages, presence
  tracking, and message history. Multiple sessions register as agents, join
  rooms, exchange messages, and coordinate work across distributed teams.
- **`nats-chat-console` — a human TUI client** (Go, in [`console/`](./console)).
  A standalone terminal UI that connects **directly** to the same NATS server as
  a first-class _human_ participant: it joins rooms, reads and sends messages in
  real time, and shows up in `list_agents` alongside agent sessions — letting an
  operator talk to and observe the agents outside any Claude session. It is a
  separate Go module, not part of the MCP server.

## Build & install

### MCP server

Not yet published to npm — build from a local checkout and link the `nats-chat`
bin onto your PATH:

```bash
git clone https://github.com/memblin/nats-chat-mcp.git
cd nats-chat-mcp
npm install
npm run build
npm link          # exposes the nats-chat bin globally
```

Then point your `.mcp.json` at it:

```json
{
  "mcpServers": {
    "nats-chat": {
      "command": "nats-chat",
      "env": { "NATS_URL": "nats://nats01.tkclabs.io:4222" }
    }
  }
}
```

Requires Node.js >= 20 and a JetStream-enabled NATS server (`-js`). See
[docs/configuration.md](./docs/configuration.md) for prerequisites and other
wiring options.

### Console (human TUI)

```bash
cd console
go install ./cmd/nats-chat-console   # puts nats-chat-console on $(go env GOPATH)/bin
nats-chat-console --identity chris --room go-virt
```

Full build/install and usage details are in
[console/README.md](./console/README.md).

## Documentation

- [docs/configuration.md](./docs/configuration.md) — prerequisites and MCP client
  configuration (`.mcp.json` variants, `NATS_URL`).
- [docs/tools.md](./docs/tools.md) — the MCP tool reference.
- [docs/workflow.md](./docs/workflow.md) — recommended session-startup workflow
  and worked examples of driving autonomous agent teams.
- [docs/development.md](./docs/development.md) — building and testing the server.
- [docs/roadmap.md](./docs/roadmap.md) — what's planned.
- [console/README.md](./console/README.md) — the `nats-chat-console` TUI.

## License

[Apache-2.0](./LICENSE)
