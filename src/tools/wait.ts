// MCP tool for blocking on inbound comms: wait_for_message.
import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import {
  getIdentity,
  getRooms,
  isRegistered,
  syncPresence,
} from "../identity.js";
import { waitForMessages } from "../stream-manager.js";
import { recordWaitResult } from "../wakeups.js";
import { jsonResult } from "./register.js";

const DEFAULT_TIMEOUT_MS = 30000;
const MAX_TIMEOUT_MS = 1800000; // 30 minutes — agents in known-long-wait states

// How often to refresh presence while blocked in a long wait. Must stay
// comfortably under the presence TTL (15 min, see stream-manager.ts) so an
// agent in a multi-minute wait doesn't lapse out of the registry mid-block.
const PRESENCE_KEEPALIVE_MS = 10 * 60 * 1000;

/**
 * Refresh presence on an interval for the duration of a blocking wait, returning
 * a stop function to clear it. Without this a wait longer than the presence TTL
 * would let the agent's TTL-backed presence lapse mid-block. Refresh failures
 * are swallowed — the next tick retries, and a single missed write is harmless.
 * Exported for unit testing the interval behavior with fake timers.
 */
export function startPresenceKeepalive(
  intervalMs: number = PRESENCE_KEEPALIVE_MS,
): () => void {
  const timer = setInterval(() => {
    void syncPresence().catch(() => {
      /* transient — the next tick (or the agent's next call) retries */
    });
  }, intervalMs);
  // The wait's awaited promise keeps the loop alive; this timer shouldn't on its
  // own, so a stray keepalive can never hold the process open.
  timer.unref?.();
  return () => clearInterval(timer);
}

export function registerWaitTools(server: McpServer): void {
  server.registerTool(
    "wait_for_message",
    {
      title: "Wait For Message",
      description:
        "Block until a message arrives on any joined room or this agent's direct inbox, then return everything received during the wait. Wakes the instant a message is delivered (push consumers, not polling). On timeout it returns an empty result so the caller can do housekeeping and wait again.",
      inputSchema: {
        timeout_ms: z
          .number()
          .int()
          .positive()
          .max(MAX_TIMEOUT_MS)
          .optional()
          .describe(
            "How long to block before returning an empty result (default 30000, max 1800000 = 30 minutes).",
          ),
      },
    },
    async ({ timeout_ms }) => {
      if (!isRegistered()) {
        throw new Error(
          "Not registered — call register_agent (and join_room to listen on a room) before waiting for messages.",
        );
      }

      const identity = getIdentity();
      const rooms = getRooms();
      const timeout = timeout_ms ?? DEFAULT_TIMEOUT_MS;

      // Keep presence fresh up front, then on an interval for the duration of
      // the block — a long wait shouldn't let this agent's TTL-backed presence
      // lapse while it sits idle waiting.
      await syncPresence();
      const stopKeepalive = startPresenceKeepalive();

      const start = Date.now();
      let roomMessages, directMessages;
      try {
        ({ roomMessages, directMessages } = await waitForMessages(
          identity.id,
          rooms,
          timeout,
        ));
      } finally {
        stopKeepalive();
      }
      const elapsed_ms = Date.now() - start;

      const timed_out =
        roomMessages.length === 0 && directMessages.length === 0;
      const consecutive_empty_wakeups = recordWaitResult(
        identity.id,
        timed_out,
      );

      return jsonResult({
        timed_out,
        elapsed_ms,
        consecutive_empty_wakeups,
        room_messages: roomMessages,
        direct_messages: directMessages,
      });
    },
  );
}
