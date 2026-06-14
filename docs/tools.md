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
  Enforced as **at most once per agent turn** per identity via single-flight
  coalescing plus a 5-second cooldown (see below).
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

### Once-per-turn enforcement

`wait_for_message` is meant to be called **at most once per agent turn** — the
turn-by-turn cycle is the loop. Prompt instructions alone don't reliably hold
this, so the server enforces it with two complementary mechanisms keyed on the
agent identity. Both are invisible to correct once-per-turn usage; they only
engage when calls pile up inside a single turn.

**Single-flight coalescing** handles calls that arrive _while a wait is already
blocking_. The first call opens the real wait; any concurrent call for the same
identity attaches to it and resolves with the same result instead of opening a
second consumer on the same durable (which would split deliveries and ack them
independently). The first caller's `timeout_ms` governs the shared wait. A
coalesced follower's result carries `"coalesced": true`.

**A 5-second cooldown** handles a call that arrives _just after_ a return. The
time and payload of each return are recorded; a call within 5 seconds of the
previous return does not start a new blocking wait. What it gets back depends on
that previous return:

- **Previous return was empty** → the same empty result is replayed
  (idempotent — an empty wait consumed nothing). The replayed copy carries
  `"replayed_from_cache": true`.
- **Previous return delivered messages** → replaying would re-show
  already-consumed messages, so the call is rejected with a self-explanatory
  result telling the agent to process what it has first:

  ```jsonc
  {
    "error": "too_soon",
    "message": "wait_for_message was called <n>ms after its previous return ...",
    "retry_after_ms": number, // ms remaining before another wait is allowed
    "consecutive_empty_wakeups": number
  }
  ```

The 5-second window is long enough that no legitimate single-turn tool sequence
hits it, and short enough to be invisible to normal turn-by-turn usage. It is a
fixed constant — not configurable. The cooldown is measured from the last
genuine return; a replayed or rejected call does not reset the clock.
`register_agent` clears both the cooldown and any in-flight-wait record for that
identity.

## `send_ack` response

```jsonc
{
  "delivered": boolean,
  "to": string, // resolved recipient id
  "regarding": string,
  "timestamp": string
}
```
