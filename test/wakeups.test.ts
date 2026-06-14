// Unit tests for the consecutive-empty-wakeup counter. Pure in-memory logic —
// no broker required.
import { afterEach, describe, expect, test } from "vitest";
import {
  coalesceWait,
  decideWaitCooldown,
  emptyWakeupCount,
  recordWaitResult,
  recordWaitReturn,
  resetEmptyWakeups,
  resetEmptyWakeupsForTests,
  resetWaitReturn,
  resetWaitReturnForTests,
  WAIT_COOLDOWN_MS,
  type WaitReturnPayload,
} from "../src/wakeups.js";

/** A wait_for_message return payload for cooldown tests. */
function payload(timed_out: boolean): WaitReturnPayload {
  return {
    timed_out,
    elapsed_ms: timed_out ? 30000 : 12,
    consecutive_empty_wakeups: timed_out ? 1 : 0,
    room_messages: [],
    direct_messages: [],
  };
}

afterEach(() => {
  resetEmptyWakeupsForTests();
  resetWaitReturnForTests();
});

describe("recordWaitResult", () => {
  test("increments on each consecutive empty wakeup", () => {
    const id = "a1";
    expect(recordWaitResult(id, true)).toBe(1);
    expect(recordWaitResult(id, true)).toBe(2);
    expect(recordWaitResult(id, true)).toBe(3);
  });

  test("resets to 0 when a wait delivers messages", () => {
    const id = "a1";
    recordWaitResult(id, true);
    recordWaitResult(id, true);
    expect(recordWaitResult(id, false)).toBe(0);
    expect(recordWaitResult(id, true)).toBe(1); // counting resumes from zero
  });

  test("a non-empty first wait stays at 0", () => {
    expect(recordWaitResult("fresh", false)).toBe(0);
  });

  test("tallies are independent per identity", () => {
    expect(recordWaitResult("a1", true)).toBe(1);
    expect(recordWaitResult("a2", true)).toBe(1);
    expect(recordWaitResult("a1", true)).toBe(2);
    expect(recordWaitResult("a2", false)).toBe(0);
    expect(recordWaitResult("a1", true)).toBe(3);
  });
});

describe("resetEmptyWakeups", () => {
  test("clears a streak — models an active receive via check_messages/check_direct", () => {
    const id = "a1";
    recordWaitResult(id, true);
    recordWaitResult(id, true);
    expect(emptyWakeupCount(id)).toBe(2);

    resetEmptyWakeups(id);
    expect(emptyWakeupCount(id)).toBe(0);

    // the next empty wakeup starts a fresh streak
    expect(recordWaitResult(id, true)).toBe(1);
  });

  test("only resets the named identity", () => {
    recordWaitResult("a1", true);
    recordWaitResult("a2", true);
    resetEmptyWakeups("a1");
    expect(emptyWakeupCount("a1")).toBe(0);
    expect(emptyWakeupCount("a2")).toBe(1);
  });
});

describe("decideWaitCooldown", () => {
  const id = "a1";

  test("a never-waited identity proceeds", () => {
    expect(decideWaitCooldown(id, 10_000)).toEqual({ action: "proceed" });
  });

  test("replays an empty result within the cooldown window", () => {
    recordWaitReturn(id, payload(true), 10_000);
    const decision = decideWaitCooldown(id, 11_000); // 1s later

    expect(decision.action).toBe("replay");
    if (decision.action !== "replay") throw new Error("unreachable");
    expect(decision.payload.timed_out).toBe(true);
    // served copy is marked, without a new blocking wait
    expect(decision.payload.replayed_from_cache).toBe(true);
  });

  test("rejects a messages-bearing result within the window (no duplicate delivery)", () => {
    recordWaitResult(id, true);
    recordWaitResult(id, true);
    recordWaitReturn(id, payload(false), 10_000); // delivered messages
    const decision = decideWaitCooldown(id, 11_000);

    expect(decision.action).toBe("reject");
    if (decision.action !== "reject") throw new Error("unreachable");
    expect(decision.payload).toMatchObject({
      error: "too_soon",
      retry_after_ms: WAIT_COOLDOWN_MS - 1000,
      consecutive_empty_wakeups: 2,
    });
    expect(decision.payload.message).toContain("1000ms");
    expect(decision.payload.message).toContain("Process those messages");
  });

  test("proceeds once the cooldown window has elapsed", () => {
    recordWaitReturn(id, payload(true), 10_000);
    expect(decideWaitCooldown(id, 10_000 + WAIT_COOLDOWN_MS).action).toBe(
      "proceed",
    );
    expect(decideWaitCooldown(id, 20_000).action).toBe("proceed");
  });

  test("the cooldown is independent per identity", () => {
    recordWaitReturn("a1", payload(true), 10_000);
    // a2 never waited, so it proceeds even while a1 is cooling down
    expect(decideWaitCooldown("a1", 11_000).action).toBe("replay");
    expect(decideWaitCooldown("a2", 11_000).action).toBe("proceed");
  });

  test("resetWaitReturn clears the cooldown — models register_agent", () => {
    recordWaitReturn(id, payload(true), 10_000);
    expect(decideWaitCooldown(id, 11_000).action).toBe("replay"); // on cooldown

    resetWaitReturn(id); // what register_agent does

    expect(decideWaitCooldown(id, 11_000).action).toBe("proceed"); // clean slate
  });
});

describe("coalesceWait (single-flight)", () => {
  test("concurrent callers share one wait; only the first is leader", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    let runs = 0;
    const start = async (): Promise<WaitReturnPayload> => {
      runs++;
      await gate;
      return payload(true);
    };

    const first = coalesceWait("a1", start);
    const second = coalesceWait("a1", start); // arrives while first blocks
    release();
    const [a, b] = await Promise.all([first, second]);

    expect(runs).toBe(1); // the wait ran exactly once
    expect(a.leader).toBe(true);
    expect(b.leader).toBe(false);
    expect(a.result).toBe(b.result); // followers mirror the leader's result
  });

  test("a call after the in-flight wait settles starts a fresh wait", async () => {
    let runs = 0;
    const start = async (): Promise<WaitReturnPayload> => {
      runs++;
      return payload(true);
    };

    const a = await coalesceWait("a1", start);
    const b = await coalesceWait("a1", start); // after the first settled

    expect(runs).toBe(2);
    expect(a.leader).toBe(true);
    expect(b.leader).toBe(true);
  });

  test("waits are coalesced per identity, not globally", async () => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    let runs = 0;
    const start = async (): Promise<WaitReturnPayload> => {
      runs++;
      await gate;
      return payload(true);
    };

    const a1 = coalesceWait("a1", start);
    const a2 = coalesceWait("a2", start); // different identity → its own wait
    release();
    const [ra1, ra2] = await Promise.all([a1, a2]);

    expect(runs).toBe(2);
    expect(ra1.leader).toBe(true);
    expect(ra2.leader).toBe(true);
  });
});
