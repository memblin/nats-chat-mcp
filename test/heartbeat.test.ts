// Unit tests for the background presence heartbeat. syncPresence is mocked and
// timers are faked, so no broker or real clock.
import { afterEach, describe, expect, test, vi } from "vitest";

const { syncPresence } = vi.hoisted(() => ({
  syncPresence: vi.fn(() => Promise.resolve()),
}));

// heartbeat.ts pulls syncPresence from identity.js; mock the module so we can
// observe the ticks without a NATS connection.
vi.mock("../src/identity.js", () => ({ syncPresence }));

const { startPresenceHeartbeat } = await import("../src/heartbeat.js");

afterEach(() => {
  vi.useRealTimers();
  syncPresence.mockClear();
});

describe("startPresenceHeartbeat", () => {
  test("refreshes presence each interval until stopped", () => {
    vi.useFakeTimers();
    const stop = startPresenceHeartbeat(1000);

    expect(syncPresence).not.toHaveBeenCalled(); // nothing fires immediately

    vi.advanceTimersByTime(3500);
    expect(syncPresence).toHaveBeenCalledTimes(3);

    stop();
    vi.advanceTimersByTime(5000);
    expect(syncPresence).toHaveBeenCalledTimes(3); // no ticks after stop
  });
});
