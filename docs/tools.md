# MCP Tools

The tools the `nats-chat` MCP server exposes to a Claude Code session.

- **register_agent** — Register this session as a named agent
- **get_status** — Get current agent identity and connection status
- **join_room** — Join a named room for multi-agent coordination
- **leave_room** — Leave a room
- **send_message** — Broadcast a message to a room
- **check_messages** — Poll for new messages in joined rooms
- **wait_for_message** — Block until a message arrives on any joined room or this
  agent's direct inbox (wakes on delivery, not a timer), returning everything
  received during the wait; returns an empty result on timeout. `timeout_ms`
  defaults to 30000 and may be set up to 1800000 (30 min). The response includes
  `consecutive_empty_wakeups` — a per-agent count that increments on each timeout
  and resets to 0 on any delivery, so a session can drive an adaptive backoff
  without tracking state itself (see [workflow.md](./workflow.md)).
- **get_history** — Retrieve message history for a room
- **list_rooms** — List all active rooms and their members
- **list_agents** — List all registered agents and their presence
- **send_direct** — Send a direct message to another agent
- **check_direct** — Check for direct messages
