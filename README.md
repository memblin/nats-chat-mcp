# claude-nats-mcp

An MCP server for inter-session communication between Claude Code instances. Built on NATS JetStream, it provides room-based messaging, direct agent communication, presence tracking, and message history. Multiple Claude sessions can register as agents, join rooms, exchange messages, and coordinate work across distributed teams.

## Installation

Add this to your `.mcp.json`:

```json
{
  "mcpServers": {
    "nats-chat": {
      "command": "npx",
      "args": ["-y", "claude-nats-mcp"],
      "env": {
        "NATS_URL": "nats://nats01.tkclabs.io:4222"
      }
    }
  }
}
```

## Available Tools

- **register_agent** — Register this session as a named agent with a role
- **get_status** — Get current agent identity and connection status
- **join_room** — Join a named room for multi-agent coordination
- **leave_room** — Leave a room
- **send_message** — Broadcast a message to a room
- **check_messages** — Poll for new messages in joined rooms
- **get_history** — Retrieve message history for a room
- **list_rooms** — List all active rooms and their members
- **list_agents** — List all registered agents and their presence
- **send_direct** — Send a direct message to another agent
- **check_direct** — Check for direct messages

## Recommended Session Startup Workflow

1. Load the `claude-nats-mcp` MCP server in your Claude session
2. Call `register_agent` with your session name and role (e.g., "build-seat-1", "validator", "lead")
3. Optionally join rooms with `join_room` (e.g., "team-sync", "release-coordination")
4. Use `send_message` to broadcast to rooms, `send_direct` for point-to-point
5. Poll for updates with `check_messages` and `check_direct` at key coordination points
6. Check `list_agents` and `list_rooms` to understand team composition
7. Call `get_history` for context on past room conversations
