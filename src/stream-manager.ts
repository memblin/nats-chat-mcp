// JetStream infrastructure + every NATS read/write the tools rely on.
// Tools call these typed helpers and never touch the NATS client directly.
import {
  AckPolicy,
  DeliverPolicy,
  DiscardPolicy,
  RetentionPolicy,
  StorageType,
  JSONCodec,
  nanos,
  type Consumer,
  type ConsumerMessages,
  type KV,
} from "nats";
import { getJetStream, getManager } from "./nats-client.js";
import type { AgentPresence, Message } from "./types.js";

// ---------------------------------------------------------------------------
// Names & subjects
// ---------------------------------------------------------------------------

const ROOM_STREAM = "CLAUDE_CHAT_ROOMS";
const DIRECT_STREAM = "CLAUDE_CHAT_DIRECT";
const PRESENCE_KV = "claude_chat_agents";

// Retain a day of room/direct traffic, capped per subject so a busy room
// can't grow without bound. Consumers expire a week after going idle.
const MESSAGE_MAX_AGE_MS = 24 * 60 * 60 * 1000;
const MESSAGE_MAX_PER_SUBJECT = 1000;
const CONSUMER_IDLE_MS = 7 * 24 * 60 * 60 * 1000;
// Linger window for presence: the server expires a record this long after its
// last write, so a crashed/abandoned session clears on its own. Kept short
// because every live participant refreshes well inside it — a registered agent
// heartbeats every 60s for the life of its session (see heartbeat.ts) and the
// console every 30s — so 5 min is several missed heartbeats. Must match the
// console's presenceTTL (console/internal/nats/client.go) so either side creates
// an identical bucket.
const PRESENCE_TTL_MS = 5 * 60 * 1000;

// After the first message wakes a blocking wait, keep gathering for this brief
// window so a burst of messages arriving a few-score ms apart all land in the
// same response rather than being split across two calls.
const WAIT_SETTLE_MS = 200;

const roomSubject = (room: string) => `chat.room.${room}.msg`;
const directSubject = (agentId: string) => `chat.direct.${agentId}.msg`;
const roomConsumerName = (agentId: string, room: string) =>
  `room_${agentId}_${room}`;
const directConsumerName = (agentId: string) => `direct_${agentId}`;

/**
 * Names used in NATS subject tokens and durable consumer names must avoid
 * `.`, `*`, `>` and whitespace. We allow a conservative slug character set.
 * Throws a user-facing error when the value is unusable.
 */
export function assertValidToken(kind: string, value: string): void {
  if (!/^[A-Za-z0-9_-]{1,64}$/.test(value)) {
    throw new Error(
      `Invalid ${kind} "${value}": use 1-64 chars of letters, digits, "_" or "-" only`,
    );
  }
}

const messageCodec = JSONCodec<Message>();
const presenceCodec = JSONCodec<AgentPresence>();

// ---------------------------------------------------------------------------
// Infrastructure bootstrap
// ---------------------------------------------------------------------------

let presenceKv: KV | null = null;

/** Create the streams and presence bucket if they don't already exist. */
export async function ensureInfrastructure(): Promise<void> {
  await ensureStream(ROOM_STREAM, ["chat.room.>"]);
  await ensureStream(DIRECT_STREAM, ["chat.direct.>"]);
  await getPresenceKv();
}

async function ensureStream(name: string, subjects: string[]): Promise<void> {
  const jsm = getManager();
  try {
    await jsm.streams.info(name);
  } catch {
    // A JetStream "stream" is a server-side log that persists every message
    // published to its subjects, so they can be replayed later.
    await jsm.streams.add({
      name,
      subjects,
      // Limits retention trims by age/count rather than waiting for every
      // consumer to acknowledge — fitting for a chat log nobody must drain.
      retention: RetentionPolicy.Limits,
      storage: StorageType.File,
      // When a subject hits max_msgs_per_subject, drop the OLDEST message.
      // Together with max_age this bounds storage even for a busy room.
      discard: DiscardPolicy.Old,
      max_age: nanos(MESSAGE_MAX_AGE_MS),
      max_msgs_per_subject: MESSAGE_MAX_PER_SUBJECT,
    });
  }
}

async function getPresenceKv(): Promise<KV> {
  if (presenceKv) return presenceKv;
  // A KV (key/value) bucket is JetStream's map abstraction — here keyed by
  // agent id. We use it as the live agent registry.
  presenceKv = await getJetStream().views.kv(PRESENCE_KV, {
    // history: 1 — keep only the latest presence record per agent (we never
    // need older versions).
    history: 1,
    // ttl — the server auto-expires an entry this long after its last write,
    // so an agent that crashes without unregistering drops off list_agents on
    // its own once it stops refreshing presence.
    ttl: PRESENCE_TTL_MS,
  });
  return presenceKv;
}

/** Drop the cached KV handle so it can't outlive a closed connection (tests). */
export function resetStreamManagerForTests(): void {
  presenceKv = null;
}

// ---------------------------------------------------------------------------
// Presence registry (KV)
// ---------------------------------------------------------------------------

export async function putPresence(presence: AgentPresence): Promise<void> {
  const kv = await getPresenceKv();
  await kv.put(presence.id, presenceCodec.encode(presence));
}

export async function getPresence(
  agentId: string,
): Promise<AgentPresence | null> {
  const kv = await getPresenceKv();
  const entry = await kv.get(agentId);
  if (entry?.operation !== "PUT") return null;
  try {
    return entry.json<AgentPresence>();
  } catch {
    return null;
  }
}

export async function deletePresence(agentId: string): Promise<void> {
  const kv = await getPresenceKv();
  await kv.delete(agentId);
}

export async function listPresence(): Promise<AgentPresence[]> {
  const kv = await getPresenceKv();
  const out: AgentPresence[] = [];
  // Drain the key iterator FULLY before fetching any value. kv.keys() and
  // kv.get() each open their own consumer on the bucket's stream; calling get()
  // while still iterating keys() interleaves the two and the key scan returns
  // only a partial set — so with several agents registered, list_agents would
  // silently drop most of them. Materialize the keys first, then get each.
  const keys: string[] = [];
  for await (const key of await kv.keys()) keys.push(key);
  for (const key of keys) {
    const entry = await kv.get(key);
    if (entry?.operation !== "PUT") continue;
    try {
      out.push(entry.json<AgentPresence>());
    } catch {
      /* skip malformed entry */
    }
  }
  return out;
}

// ---------------------------------------------------------------------------
// Publishing
// ---------------------------------------------------------------------------

export async function publishRoomMessage(
  room: string,
  message: Message,
): Promise<void> {
  await getJetStream().publish(roomSubject(room), messageCodec.encode(message));
}

export async function publishDirectMessage(
  toAgentId: string,
  message: Message,
): Promise<void> {
  await getJetStream().publish(
    directSubject(toAgentId),
    messageCodec.encode(message),
  );
}

// ---------------------------------------------------------------------------
// Durable consumers — one per (agent, room) and one per agent for DMs.
//
// A "durable consumer" is a named read cursor stored ON THE NATS SERVER. Because
// the server remembers each agent's position, a Claude session can disconnect
// and reconnect without losing its place: each check_* call fetches only what
// arrived since the last one was acknowledged. That server-side cursor is what
// makes check_messages / check_direct incremental rather than re-reading
// everything every time.
// ---------------------------------------------------------------------------

export async function ensureRoomConsumer(
  agentId: string,
  room: string,
): Promise<void> {
  await ensureConsumer(
    ROOM_STREAM,
    roomConsumerName(agentId, room),
    roomSubject(room),
  );
}

export async function deleteRoomConsumer(
  agentId: string,
  room: string,
): Promise<void> {
  try {
    await getManager().consumers.delete(
      ROOM_STREAM,
      roomConsumerName(agentId, room),
    );
  } catch {
    /* already gone */
  }
}

export async function ensureDirectConsumer(agentId: string): Promise<void> {
  await ensureConsumer(
    DIRECT_STREAM,
    directConsumerName(agentId),
    directSubject(agentId),
  );
}

async function ensureConsumer(
  stream: string,
  durable: string,
  filterSubject: string,
): Promise<void> {
  const jsm = getManager();
  try {
    await jsm.consumers.info(stream, durable);
    return;
  } catch {
    /* needs creating */
  }
  await jsm.consumers.add(stream, {
    durable_name: durable,
    // Explicit: the cursor only advances when we call msg.ack(); nothing is
    // auto-confirmed. This is what lets drainConsumer below decide what counts
    // as "delivered".
    ack_policy: AckPolicy.Explicit,
    // New: start from messages published AFTER this consumer is created, so an
    // agent joining a room late doesn't get flooded with old backlog (use
    // get_history for that).
    deliver_policy: DeliverPolicy.New,
    // Only deliver this agent's slice of the stream (its room, or its DMs).
    filter_subject: filterSubject,
    // The server deletes this consumer if it goes unused this long, so cursors
    // for vanished agents don't pile up forever.
    inactive_threshold: nanos(CONSUMER_IDLE_MS),
  });
}

export async function fetchRoomMessages(
  agentId: string,
  room: string,
  max = 50,
): Promise<Message[]> {
  const consumer = await getJetStream().consumers.get(
    ROOM_STREAM,
    roomConsumerName(agentId, room),
  );
  return drainConsumer(consumer, max, agentId);
}

export async function fetchDirectMessages(
  agentId: string,
  max = 50,
): Promise<Message[]> {
  const consumer = await getJetStream().consumers.get(
    DIRECT_STREAM,
    directConsumerName(agentId),
  );
  return drainConsumer(consumer, max, agentId);
}

async function drainConsumer(
  consumer: Consumer,
  max: number,
  selfId: string,
): Promise<Message[]> {
  const out: Message[] = [];
  // fetch pulls up to `max` messages, waiting at most `expires` ms for any to
  // arrive before returning (so an empty room returns quickly, not hanging).
  const batch = await consumer.fetch({ max_messages: max, expires: 1000 });
  for await (const msg of batch) {
    let parsed: Message | undefined;
    try {
      parsed = msg.json<Message>();
    } catch {
      /* skip malformed payload */
    }
    // ack() advances this consumer's server-side cursor past the message even
    // when we drop it — without it the same messages would be re-delivered on
    // the next fetch. We ack the agent's OWN echo too, so it never lingers.
    msg.ack();
    // The agent's own posts echo back on its room consumer (same subject). Hand
    // them back and the caller sees itself as fresh traffic — so filter them.
    if (parsed && parsed.from_id !== selfId) out.push(parsed);
  }
  return out;
}

// ---------------------------------------------------------------------------
// Blocking wait — wake on first delivery across all of an agent's subjects.
//
// Where fetchRoomMessages / fetchDirectMessages do a single bounded pull and
// return whatever is queued right now, waitForMessages BLOCKS until something
// arrives (or the timeout fires). It does this by opening a push-style
// continuous read (consume()) on each of the agent's existing durable
// consumers, so the server delivers the moment a message is published rather
// than on a polling timer. Because it rides the SAME durable consumers that
// the check_* tools drain — and acks every message it returns — a message is
// never handed back by both this and a later check_messages / check_direct.
// ---------------------------------------------------------------------------

export interface WaitResult {
  roomMessages: Message[];
  directMessages: Message[];
}

export async function waitForMessages(
  agentId: string,
  rooms: string[],
  timeoutMs: number,
): Promise<WaitResult> {
  const js = getJetStream();
  const roomMessages: Message[] = [];
  const directMessages: Message[] = [];

  // Resolve `firstMessage` the instant any subject delivers, so the wait wakes
  // on delivery instead of running the clock down.
  let wake: () => void = () => {};
  const firstMessage = new Promise<void>((resolve) => {
    wake = resolve;
  });

  const subscriptions: ConsumerMessages[] = [];

  // Open a continuous read on one durable consumer, draining each delivery into
  // `bucket` and acking it (which advances the shared server-side cursor so the
  // message won't be re-delivered to a later check_* call).
  const pump = async (consumer: Consumer, bucket: Message[]): Promise<void> => {
    const iter = await consumer.consume({ max_messages: 100 });
    subscriptions.push(iter);
    try {
      for await (const msg of iter) {
        let parsed: Message | undefined;
        try {
          parsed = msg.json<Message>();
        } catch {
          /* skip malformed payload */
        }
        // Ack unconditionally so the cursor advances past anything delivered —
        // including the agent's own echo, which we otherwise drop.
        msg.ack();
        // A self-authored message is the agent hearing itself: don't surface it
        // and, crucially, don't wake() — otherwise the agent's own send would
        // resolve its next wait_for_message and reset the empty-wakeup streak,
        // making a quiet room look busy.
        if (parsed && parsed.from_id !== agentId) {
          bucket.push(parsed);
          wake();
        }
      }
    } catch {
      /* iterator stopped or consumer went away — nothing left to gather */
    }
  };

  const roomConsumers = await Promise.all(
    rooms.map((room) =>
      js.consumers.get(ROOM_STREAM, roomConsumerName(agentId, room)),
    ),
  );
  const directConsumer = await js.consumers.get(
    DIRECT_STREAM,
    directConsumerName(agentId),
  );

  const pumps = [
    ...roomConsumers.map((c) => pump(c, roomMessages)),
    pump(directConsumer, directMessages),
  ];

  let timer: ReturnType<typeof setTimeout> | undefined;
  const timeout = new Promise<void>((resolve) => {
    timer = setTimeout(resolve, timeoutMs);
  });

  await Promise.race([firstMessage, timeout]);

  // Woken by a delivery? Linger briefly to sweep up a closely-following burst.
  if (roomMessages.length > 0 || directMessages.length > 0) {
    await new Promise<void>((resolve) => setTimeout(resolve, WAIT_SETTLE_MS));
  }

  if (timer) clearTimeout(timer);
  for (const iter of subscriptions) iter.stop();
  await Promise.allSettled(pumps);

  return { roomMessages, directMessages };
}

// ---------------------------------------------------------------------------
// History — read-only replay over a room's retained messages.
// ---------------------------------------------------------------------------

export async function getRoomHistory(
  room: string,
  limit = 50,
): Promise<Message[]> {
  // Unlike the durable consumers above, history uses an ORDERED (ephemeral)
  // consumer: a throwaway, client-managed cursor that always replays from the
  // beginning of the stream and isn't stored on the server — perfect for a
  // one-shot read. It already replays from the start and manages its own start
  // sequence, so we must NOT also set deliver_policy, or the server rejects it
  // ("deliver all, but optional start sequence is set").
  const consumer = await getJetStream().consumers.get(ROOM_STREAM, {
    filterSubjects: roomSubject(room),
  });
  const out: Message[] = [];
  const batch = await consumer.fetch({
    max_messages: MESSAGE_MAX_PER_SUBJECT,
    expires: 1500,
  });
  for await (const msg of batch) {
    try {
      out.push(msg.json<Message>());
    } catch {
      /* skip malformed payload */
    }
  }
  return out.slice(-limit);
}
