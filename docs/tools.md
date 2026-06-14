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
  `consecutive_empty_wakeups` (see below), so a session can drive an adaptive
  backoff without tracking state itself (see [workflow.md](./workflow.md)).
- **get_history** — Retrieve message history for a room
- **list_rooms** — List all active rooms and their members
- **list_agents** — List all registered agents and their presence
- **send_direct** — Send a direct message to another agent
- **send_ack** — Acknowledge a message you received and are acting on: a
  lightweight status ping to the sender rather than a full reply. Delivered on
  the same direct channel as `send_direct` but tagged `type: "ack"` so the
  console and recipients can render it as a status badge. Call it immediately on
  wakeup, before doing work, so the sender gets delivery confirmation within
  seconds. Inputs: `to` (name or id), `regarding` (brief label), `status` (one of
  `received` | `investigating` | `dispatching` | `in_progress` | `blocked` |
  `complete`), and optional `note` (max 200 chars).
- **check_direct** — Check for direct messages

## `wait_for_message` response

```jsonc
{
  "timed_out": boolean,
  "elapsed_ms": number,
  "consecutive_empty_wakeups": number, // per-agent streak of empty wakeups
  "room_messages": Message[],
  "direct_messages": Message[]
}
```

`consecutive_empty_wakeups` increments each time `wait_for_message` returns with
no messages and resets to 0 on any active receive — a `wait_for_message` that
delivered, or a `check_messages` / `check_direct` that returned messages. It is
tracked per agent identity in the server's memory.

## `send_ack` response

```jsonc
{
  "delivered": boolean,
  "to": string, // resolved recipient id
  "regarding": string,
  "timestamp": string
}
```
