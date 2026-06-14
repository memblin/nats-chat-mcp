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
import {
  coalesceWait,
  decideWaitCooldown,
  recordWaitResult,
  recordWaitReturn,
} from "../wakeups.js";
import { jsonResult } from "./register.js";

const DEFAULT_TIMEOUT_MS = 30000;
const MAX_TIMEOUT_MS = 1800000; // 30 minutes — agents in known-long-wait states

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

      // Per-identity cooldown gate, checked before any work. Within the window:
      // an empty previous return is replayed (idempotent — nothing was
      // consumed), and a messages-bearing one is rejected with guidance. This
      // is what stops a single turn from stacking wait_for_message calls.
      const decision = decideWaitCooldown(identity.id);
      if (decision.action === "replay" || decision.action === "reject") {
        return jsonResult(decision.payload);
      }

      const rooms = getRooms();
      const timeout = timeout_ms ?? DEFAULT_TIMEOUT_MS;

      // Single-flight: if a wait is already blocking for this identity (a burst
      // of calls fired in one turn before the first returned), attach to it
      // rather than opening a second consumer on the same durable. Only the
      // leader records streak/return state; followers mirror its result so one
      // wait is never counted many times. The first caller's timeout governs.
      const { leader, result } = await coalesceWait(identity.id, async () => {
        // Keep presence fresh up front; the session's background heartbeat
        // keeps it alive for the rest of a long block (see heartbeat.ts).
        await syncPresence();

        const start = Date.now();
        const { roomMessages, directMessages } = await waitForMessages(
          identity.id,
          rooms,
          timeout,
        );
        const elapsed_ms = Date.now() - start;

        const timed_out =
          roomMessages.length === 0 && directMessages.length === 0;
        const consecutive_empty_wakeups = recordWaitResult(
          identity.id,
          timed_out,
        );

        const payload = {
          timed_out,
          elapsed_ms,
          consecutive_empty_wakeups,
          room_messages: roomMessages,
          direct_messages: directMessages,
        };

        // Stamp + cache the return so the cooldown is measured from this moment
        // and an empty result can be replayed within the window.
        recordWaitReturn(identity.id, payload);
        return payload;
      });

      // Mark a coalesced follower's copy so it's observable in logs; the leader
      // returns the result verbatim.
      return jsonResult(leader ? result : { ...result, coalesced: true });
    },
  );
}
