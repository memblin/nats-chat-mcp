// Per-identity tally of consecutive empty wakeups, kept in process memory (no
// persistence). An "empty wakeup" is a wait_for_message that timed out with no
// messages. ANY active receive — a wait_for_message that delivered, or a
// check_messages / check_direct that returned messages — resets the streak. This
// lets an agent drive an adaptive backoff (tighten when busy, relax when quiet)
// without tracking the streak itself.

const emptyWakeups = new Map<string, number>();

// The shape wait_for_message hands back on a real return; cached per identity so
// the cooldown can replay an empty one (see decideWaitCooldown). Carries at
// least `timed_out`; the rest rides along opaquely so we can replay it verbatim.
export interface WaitReturnPayload {
  timed_out: boolean;
  [key: string]: unknown;
}

// Per-identity record of the most recent wait_for_message return: when it
// returned and what it returned. Backs two things — a hard cooldown so an agent
// can't fire wait_for_message repeatedly inside a single turn (the prompt
// instruction to call it once per turn isn't enough on its own), and the
// idempotent replay of an empty result within that window. The turn-by-turn
// cycle IS the loop.
const lastWait = new Map<string, { at: number; payload: WaitReturnPayload }>();

// Per-identity in-flight wait promise, for single-flight coalescing: a second
// wait_for_message that arrives WHILE one is already blocking attaches to it
// rather than opening a second consumer on the same durable (which would split
// deliveries and ack them independently). See coalesceWait.
const inFlightWaits = new Map<string, Promise<WaitReturnPayload>>();

/**
 * Cooldown window between successive wait_for_message returns for one identity.
 * A hard constant on purpose — long enough that no legitimate single-turn
 * tool-call sequence hits it, short enough to be invisible to normal
 * turn-by-turn usage (turns take seconds). Not configurable by design.
 */
export const WAIT_COOLDOWN_MS = 5000;

/** The "too_soon" payload returned when a wait is rejected by the cooldown. */
export interface WaitCooldownError {
  error: "too_soon";
  message: string;
  retry_after_ms: number;
  consecutive_empty_wakeups: number;
}

/**
 * Decide what a wait_for_message call should do given the cooldown state for
 * its identity. Pure — `now` is injectable for tests. Three outcomes:
 *
 *  - `proceed`: no recent return (or the window has elapsed) — run the wait.
 *  - `replay`:  within the window AND the previous return was empty — hand back
 *               that same empty result instead of starting another blocking
 *               wait. Safe because an empty wait consumed nothing.
 *  - `reject`:  within the window AND the previous return delivered messages —
 *               replaying would re-show already-consumed messages, so return a
 *               self-explanatory "too_soon" result telling the agent to process
 *               what it has before waiting again.
 */
export type WaitCooldownDecision =
  | { action: "proceed" }
  | { action: "replay"; payload: WaitReturnPayload }
  | { action: "reject"; payload: WaitCooldownError };

export function decideWaitCooldown(
  id: string,
  now: number = Date.now(),
): WaitCooldownDecision {
  const last = lastWait.get(id);
  if (!last) return { action: "proceed" };
  const elapsed = now - last.at;
  if (elapsed >= WAIT_COOLDOWN_MS) return { action: "proceed" };

  if (last.payload.timed_out) {
    // Idempotent replay of the empty result; mark the served copy so it's
    // observable in logs without mutating the cached original.
    return {
      action: "replay",
      payload: { ...last.payload, replayed_from_cache: true },
    };
  }

  return {
    action: "reject",
    payload: {
      error: "too_soon",
      message:
        `wait_for_message was called ${elapsed}ms after its previous return ` +
        `for this identity, which delivered messages. Process those messages ` +
        `before waiting again — each wait must be in a separate agent turn. ` +
        `The turn-by-turn cycle is the loop; do not call wait_for_message more ` +
        `than once per turn.`,
      retry_after_ms: WAIT_COOLDOWN_MS - elapsed,
      consecutive_empty_wakeups: emptyWakeups.get(id) ?? 0,
    },
  };
}

/**
 * Record a real wait_for_message return for an identity: stamps the time and
 * caches the payload so decideWaitCooldown can gate (and replay) the window.
 * Called once, immediately before returning, for every genuine return.
 */
export function recordWaitReturn(
  id: string,
  payload: WaitReturnPayload,
  now: number = Date.now(),
): void {
  lastWait.set(id, { at: now, payload });
}

/**
 * Drop an identity's wait-return record — called on register_agent so a session
 * re-registering starts with a clean cooldown.
 */
export function resetWaitReturn(id: string): void {
  lastWait.delete(id);
}

/**
 * Single-flight a blocking wait for an identity. If no wait is in flight, run
 * `start` and register its promise as the in-flight wait; concurrent callers
 * that arrive before it settles attach to the SAME promise instead of starting
 * their own. Returns `{ leader, result }` — `leader` is true only for the call
 * that actually ran the wait, so the caller knows whether to record return /
 * streak state (the leader does; followers must not, or one wait would count
 * many times). The in-flight slot clears once the wait settles.
 */
export async function coalesceWait(
  id: string,
  start: () => Promise<WaitReturnPayload>,
): Promise<{ leader: boolean; result: WaitReturnPayload }> {
  const existing = inFlightWaits.get(id);
  if (existing) {
    return { leader: false, result: await existing };
  }
  const promise = start();
  inFlightWaits.set(id, promise);
  try {
    return { leader: true, result: await promise };
  } finally {
    inFlightWaits.delete(id);
  }
}

/**
 * Record a wait_for_message result for an identity: increment the streak on an
 * empty (timed-out) wait, reset it to 0 when the wait delivered messages.
 * Returns the updated tally.
 */
export function recordWaitResult(id: string, timedOut: boolean): number {
  const next = timedOut ? (emptyWakeups.get(id) ?? 0) + 1 : 0;
  emptyWakeups.set(id, next);
  return next;
}

/**
 * Reset an identity's streak to 0 — called when an agent actively receives
 * messages outside of wait_for_message (check_messages / check_direct).
 */
export function resetEmptyWakeups(id: string): void {
  emptyWakeups.set(id, 0);
}

/** Current streak for an identity (0 if untracked). */
export function emptyWakeupCount(id: string): number {
  return emptyWakeups.get(id) ?? 0;
}

/** Test-only: clear all tallies so cases don't bleed into each other. */
export function resetEmptyWakeupsForTests(): void {
  emptyWakeups.clear();
}

/** Test-only: clear all wait-return records and in-flight waits between cases. */
export function resetWaitReturnForTests(): void {
  lastWait.clear();
  inFlightWaits.clear();
}
