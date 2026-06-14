// End-to-end checks against a real JetStream broker: the foundation helpers
// that can't be exercised without a live NATS server (publish/consume/KV/history).
import { afterAll, beforeAll, describe, expect, test } from "vitest";
import { startNats, resetClientState, type NatsHandle } from "../helpers.js";
import { connectNats } from "../../src/nats-client.js";
import {
  ensureInfrastructure,
  ensureRoomConsumer,
  fetchRoomMessages,
  publishRoomMessage,
  getRoomHistory,
  ensureDirectConsumer,
  fetchDirectMessages,
  publishDirectMessage,
  listPresence,
  waitForMessages,
} from "../../src/stream-manager.js";
import {
  register,
  newAck,
  newMessage,
  syncPresence,
  resetIdentityForTests,
} from "../../src/identity.js";

let nats: NatsHandle;
let roomSeq = 0;
const uniqueRoom = () => `room-${++roomSeq}`;
const sleep = (ms: number) => new Promise<void>((r) => setTimeout(r, ms));

beforeAll(async () => {
  nats = await startNats();
  await connectNats(nats.url);
  await ensureInfrastructure();
});

afterAll(async () => {
  await resetClientState();
  if (nats) await nats.container.stop();
});

describe("room messaging", () => {
  test("a joined agent receives a broadcast, then the consumer is drained", async () => {
    const room = uniqueRoom();
    const me = await register("alice");
    await ensureRoomConsumer(me.id, room);

    // A different agent broadcasts — an agent's own posts are filtered from its
    // reads, so the sender must be a distinct identity, not this session.
    resetIdentityForTests();
    await register("peer");
    await publishRoomMessage(room, newMessage("hello room", { room }));

    const first = await fetchRoomMessages(me.id, room);
    expect(first.map((m) => m.content)).toContain("hello room");

    // check_messages is incremental: a second poll with nothing new is empty.
    const second = await fetchRoomMessages(me.id, room);
    expect(second).toHaveLength(0);
  });

  test("a message published before joining is NOT delivered (deliver new)", async () => {
    const room = uniqueRoom();
    const me = await register("bob");

    await publishRoomMessage(room, newMessage("before join", { room }));
    await ensureRoomConsumer(me.id, room); // consumer created after the publish

    const msgs = await fetchRoomMessages(me.id, room);
    expect(msgs).toHaveLength(0);
  });
});

describe("history", () => {
  test("get_history replays retained messages oldest-first and honors limit", async () => {
    const room = uniqueRoom();
    await register("carol");

    for (let i = 1; i <= 5; i++) {
      await publishRoomMessage(room, newMessage(`m${i}`, { room }));
    }

    const all = await getRoomHistory(room, 50);
    expect(all.map((m) => m.content)).toEqual(["m1", "m2", "m3", "m4", "m5"]);

    const last2 = await getRoomHistory(room, 2);
    expect(last2.map((m) => m.content)).toEqual(["m4", "m5"]);
  });
});

describe("direct messages", () => {
  test("a DM reaches the target agent's inbox only", async () => {
    const recipient = await register("dave");
    await ensureDirectConsumer(recipient.id);
    const recipientId = recipient.id;

    // The sender is a separate agent session (distinct id), then DMs the
    // recipient — a DM from the recipient itself would be filtered as self-echo.
    resetIdentityForTests();
    await register("erin");
    await publishDirectMessage(recipientId, newMessage("ping dave"));

    const inbox = await fetchDirectMessages(recipientId);
    expect(inbox.map((m) => m.content)).toContain("ping dave");
    expect(inbox.every((m) => m.from === "erin")).toBe(true);
  });
});

describe("wait_for_message (blocking wait)", () => {
  test("wakes immediately when a room message arrives during the wait", async () => {
    const room = uniqueRoom();
    const me = await register("grace");
    // register_agent + join_room create these in production; mirror that so the
    // wait has both its room consumer and its direct inbox to listen on.
    await ensureDirectConsumer(me.id);
    await ensureRoomConsumer(me.id, room);

    // Begin blocking, then publish once the push consumer has attached. The
    // sender is a different agent — a self-authored post wouldn't wake the wait.
    const waiting = waitForMessages(me.id, [room], 5000);
    await sleep(250);
    resetIdentityForTests();
    await register("peer");
    await publishRoomMessage(room, newMessage("wake up", { room }));

    const result = await waiting;
    expect(result.roomMessages.map((m) => m.content)).toContain("wake up");
    expect(result.directMessages).toHaveLength(0);
  });

  test("wakes when a direct message arrives during the wait", async () => {
    const recipient = await register("heidi");
    await ensureDirectConsumer(recipient.id);
    const recipientId = recipient.id;

    const waiting = waitForMessages(recipientId, [], 5000);
    await sleep(250);
    // A different session (distinct id) sends the DM to heidi's inbox.
    resetIdentityForTests();
    await register("ivan");
    await publishDirectMessage(recipientId, newMessage("dm wake"));

    const result = await waiting;
    expect(result.directMessages.map((m) => m.content)).toContain("dm wake");
    expect(result.roomMessages).toHaveLength(0);
  });

  test("returns an empty result after a clean timeout", async () => {
    const room = uniqueRoom();
    const me = await register("judy");
    await ensureDirectConsumer(me.id);
    await ensureRoomConsumer(me.id, room);

    const start = Date.now();
    const result = await waitForMessages(me.id, [room], 600);
    const elapsed = Date.now() - start;

    expect(result.roomMessages).toHaveLength(0);
    expect(result.directMessages).toHaveLength(0);
    // It blocked for roughly the timeout rather than returning instantly.
    expect(elapsed).toBeGreaterThanOrEqual(550);
  });

  test("accumulates several messages that arrive close together", async () => {
    const room = uniqueRoom();
    const me = await register("kevin");
    await ensureDirectConsumer(me.id);
    await ensureRoomConsumer(me.id, room);

    // Both messages come from another agent, arriving close together.
    resetIdentityForTests();
    await register("peer");
    const waiting = waitForMessages(me.id, [room], 5000);
    await sleep(250);
    await publishRoomMessage(room, newMessage("first", { room }));
    await sleep(100);
    await publishRoomMessage(room, newMessage("second", { room }));

    const result = await waiting;
    expect(result.roomMessages.map((m) => m.content)).toEqual(
      expect.arrayContaining(["first", "second"]),
    );
  });
});

describe("presence registry (KV)", () => {
  test("syncPresence makes an agent visible via listPresence", async () => {
    const me = await register("frank");
    await syncPresence();

    const agents = await listPresence();
    const self = agents.find((a) => a.id === me.id);
    expect(self).toBeDefined();
    expect(self!.name).toBe("frank");
    expect(self!.last_seen).toBeTruthy();
  });

  test("lists every distinct registered agent, not just one", async () => {
    // Three separate agent sessions (distinct ids) each publish presence.
    const ids: string[] = [];
    for (const name of ["uma", "victor", "wendy"]) {
      resetIdentityForTests();
      const a = await register(name);
      ids.push(a.id);
    }

    const agents = await listPresence();
    // Every one of the three must be visible — listPresence must not drop
    // entries when several distinct ids are registered at once.
    for (const id of ids) {
      expect(agents.some((a) => a.id === id)).toBe(true);
    }
  });
});

describe("send_ack (direct ack envelope)", () => {
  test("delivers an ack envelope (type: 'ack') to the recipient's direct inbox", async () => {
    // Sender's identity stamps the ack's from/from_id.
    await register("acker");

    // A distinct recipient inbox; create its consumer so the ack is captured.
    const recipientId = "arecipientackinbox";
    await ensureDirectConsumer(recipientId);

    const ack = newAck("rc13 publish handoff", "investigating", "on it");
    expect(ack.type).toBe("ack");
    expect(ack.regarding).toBe("rc13 publish handoff");
    expect(ack.status).toBe("investigating");
    expect(ack.note).toBe("on it");

    await publishDirectMessage(recipientId, ack);

    const messages = await fetchDirectMessages(recipientId);
    expect(messages).toHaveLength(1);
    // Extra ack fields survive the round-trip on the direct subject.
    const received = messages[0] as unknown as {
      type?: string;
      regarding?: string;
      status?: string;
      from: string;
    };
    expect(received.type).toBe("ack");
    expect(received.regarding).toBe("rc13 publish handoff");
    expect(received.status).toBe("investigating");
    expect(received.from).toBe("acker");
  });
});

describe("self-echo filtering", () => {
  test("check_messages drops the agent's own broadcast but keeps others, and acks the echo", async () => {
    const room = uniqueRoom();
    const me = await register("nadia");
    await ensureRoomConsumer(me.id, room);

    // A self-authored broadcast (from_id === me.id) — the agent's own echo.
    await publishRoomMessage(room, newMessage("hearing myself", { room }));

    // Someone else broadcasts to the same room afterward. Reset identity first
    // so this is a genuinely different session id, not a same-session rename
    // (register keeps the existing id when re-registering).
    resetIdentityForTests();
    const other = await register("mallory");
    expect(other.id).not.toBe(me.id);
    await publishRoomMessage(room, newMessage("from someone else", { room }));

    // Only the other agent's message comes back — the self-echo is filtered.
    const msgs = await fetchRoomMessages(me.id, room);
    expect(msgs.map((m) => m.content)).toEqual(["from someone else"]);

    // The echo was acked (not left pending): a second poll is empty rather than
    // re-delivering the skipped message.
    const second = await fetchRoomMessages(me.id, room);
    expect(second).toHaveLength(0);
  });

  test("wait_for_message does NOT wake on the agent's own broadcast", async () => {
    const room = uniqueRoom();
    const me = await register("ned");
    await ensureDirectConsumer(me.id);
    await ensureRoomConsumer(me.id, room);

    const start = Date.now();
    const waiting = waitForMessages(me.id, [room], 800);
    await sleep(250);
    // Only the agent's own message lands during the wait — it must be ignored.
    await publishRoomMessage(room, newMessage("just me talking", { room }));

    const result = await waiting;
    const elapsed = Date.now() - start;

    expect(result.roomMessages).toHaveLength(0);
    expect(result.directMessages).toHaveLength(0);
    // It ran the timeout down rather than waking early on the echo.
    expect(elapsed).toBeGreaterThanOrEqual(750);
  });
});
