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
import { register, newMessage, syncPresence } from "../../src/identity.js";

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

    // sender re-registers this session as someone else, then DMs the recipient
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

    // Begin blocking, then publish once the push consumer has attached.
    const waiting = waitForMessages(me.id, [room], 5000);
    await sleep(250);
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
    // A different session sends the DM to heidi's inbox.
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
});
