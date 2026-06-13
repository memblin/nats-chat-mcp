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
} from "../../src/stream-manager.js";
import { register, newMessage, syncPresence } from "../../src/identity.js";

let nats: NatsHandle;
let roomSeq = 0;
const uniqueRoom = () => `room-${++roomSeq}`;

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
