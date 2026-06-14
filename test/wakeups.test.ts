// Unit tests for the consecutive-empty-wakeup counter. Pure in-memory logic —
// no broker required.
import { afterEach, describe, expect, test } from "vitest";
import {
  emptyWakeupCount,
  recordWaitResult,
  resetEmptyWakeups,
  resetEmptyWakeupsForTests,
} from "../src/wakeups.js";

afterEach(() => resetEmptyWakeupsForTests());

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
