// Unit tests for the consecutive-empty-wakeup counter in wait_for_message.
// Pure in-memory logic — no broker required.
import { afterEach, describe, expect, test } from "vitest";
import {
  resetEmptyWakeupsForTests,
  trackEmptyWakeups,
} from "../src/tools/wait.js";

afterEach(() => resetEmptyWakeupsForTests());

describe("trackEmptyWakeups", () => {
  test("increments on each consecutive timeout", () => {
    const id = "a1";
    expect(trackEmptyWakeups(id, true)).toBe(1);
    expect(trackEmptyWakeups(id, true)).toBe(2);
    expect(trackEmptyWakeups(id, true)).toBe(3);
  });

  test("resets to 0 when a wait delivers messages", () => {
    const id = "a1";
    trackEmptyWakeups(id, true);
    trackEmptyWakeups(id, true);
    expect(trackEmptyWakeups(id, false)).toBe(0);
    // and counting resumes from zero afterwards
    expect(trackEmptyWakeups(id, true)).toBe(1);
  });

  test("a non-empty first wait stays at 0", () => {
    expect(trackEmptyWakeups("fresh", false)).toBe(0);
  });

  test("tallies are independent per identity", () => {
    expect(trackEmptyWakeups("a1", true)).toBe(1);
    expect(trackEmptyWakeups("a2", true)).toBe(1);
    expect(trackEmptyWakeups("a1", true)).toBe(2);
    expect(trackEmptyWakeups("a2", false)).toBe(0);
    expect(trackEmptyWakeups("a1", true)).toBe(3);
  });
});
