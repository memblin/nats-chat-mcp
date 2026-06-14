// Per-identity tally of consecutive empty wakeups, kept in process memory (no
// persistence). An "empty wakeup" is a wait_for_message that timed out with no
// messages. ANY active receive — a wait_for_message that delivered, or a
// check_messages / check_direct that returned messages — resets the streak. This
// lets an agent drive an adaptive backoff (tighten when busy, relax when quiet)
// without tracking the streak itself.

const emptyWakeups = new Map<string, number>();

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
