// Unit tests for the presence keepalive used during long blocking waits.
// syncPresence is mocked and timers are faked, so no broker or real clock.
import { afterEach, describe, expect, test, vi } from "vitest";

const { syncPresence } = vi.hoisted(() => ({
  syncPresence: vi.fn(() => Promise.resolve()),
}));

// wait.ts pulls these from identity.js; only syncPresence is exercised here, but
// the others must exist so the module imports cleanly.
vi.mock("../src/identity.js", () => ({
  syncPresence,
  getIdentity: vi.fn(),
  getRooms: vi.fn(),
  isRegistered: vi.fn(),
}));

const { startPresenceKeepalive } = await import("../src/tools/wait.js");

afterEach(() => {
  vi.useRealTimers();
  syncPresence.mockClear();
});

describe("startPresenceKeepalive", () => {
  test("re-syncs presence each interval until stopped", () => {
    vi.useFakeTimers();
    const stop = startPresenceKeepalive(1000);

    // Nothing up front — the handler does the initial sync itself.
    expect(syncPresence).not.toHaveBeenCalled();

    vi.advanceTimersByTime(3500);
    expect(syncPresence).toHaveBeenCalledTimes(3);

    stop();
    vi.advanceTimersByTime(5000);
    expect(syncPresence).toHaveBeenCalledTimes(3); // no ticks after stop
  });

  test("a short wait that stops before the first interval never syncs", () => {
    vi.useFakeTimers();
    const stop = startPresenceKeepalive(1000);
    vi.advanceTimersByTime(500);
    stop();
    expect(syncPresence).not.toHaveBeenCalled();
  });
});
