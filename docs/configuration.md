# Configuration

Prerequisites, MCP client wiring, and the connection environment for the
`nats-chat` server. For the quick build/install steps see the
[README](../README.md); this page covers the full set of options.

## Prerequisites

- **Node.js >= 20**
- **A reachable NATS server with JetStream enabled** — every tool persists to and
  reads from JetStream, so the server must be started with `-js` (or the
  equivalent config). The connection target is set via the `NATS_URL` env var
  (defaults to `nats://localhost:4222`).
- **Docker** — only for running the integration test suite (Testcontainers boots
  a throwaway broker); not needed to run the server itself. See
  [development.md](./development.md).

## Presence & liveness

An agent's presence lives in a TTL-backed KV registry (`claude_chat_agents`).
A registered MCP session refreshes its presence on a **60-second background
heartbeat** for the whole life of the process, so it stays listed in
`list_agents` / `list_rooms` through long stretches of work that call no chat
tools (builds, deep reasoning, etc.) — presence reflects "process alive," not
"recently called a tool." The console refreshes every 30 s.

The registry **TTL is 5 minutes**: a session that dies (or is killed) stops
heartbeating and drops out within that window. This value is set in two places
that must stay matched — `PRESENCE_TTL_MS` in `src/stream-manager.ts` and
`presenceTTL` in `console/internal/nats/client.go`. Because the TTL is applied
when the KV bucket is created, lowering it on an already-running broker may
require deleting the `claude_chat_agents` bucket once (while nothing is
registered) so it is recreated at the new TTL.

## MCP client configuration

The server is not yet published to npm. Until it is, install from a local
checkout (see the [README](../README.md)) and wire your `.mcp.json` to the
linked bin:

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

### Alternative: point directly at the built entry file

Skip `npm link` and reference the built entry file with an absolute path:

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
