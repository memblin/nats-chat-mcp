// Background presence heartbeat. A registered agent otherwise only refreshes its
// presence when it calls a chat tool, so a session doing a long stretch of work
// that touches no nats-chat tool would lapse out of the registry (and look
// "gone" to peers) even though its process is alive. This refreshes presence on
// a fixed interval for the whole life of the session, so presence reflects
// "process alive" rather than "recently called a tool". The interval must stay
// comfortably under the presence TTL (see PRESENCE_TTL_MS in stream-manager.ts).
import { syncPresence } from "./identity.js";

const PRESENCE_HEARTBEAT_MS = 60_000; // 1 min — well under the 5-min presence TTL

/**
 * Start refreshing presence every intervalMs until the returned stop function is
 * called. syncPresence no-ops until the session registers, so this is safe to
 * start at boot. Refresh failures are swallowed — the next tick retries, and one
 * missed write is harmless given the TTL headroom.
 */
export function startPresenceHeartbeat(
  intervalMs: number = PRESENCE_HEARTBEAT_MS,
): () => void {
  const timer = setInterval(() => {
    void syncPresence().catch(() => {
      /* transient — the next tick retries */
    });
  }, intervalMs);
  // Don't let the heartbeat alone keep the process alive.
  timer.unref?.();
  return () => clearInterval(timer);
}
