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
import { jsonResult } from "./register.js";

const DEFAULT_TIMEOUT_MS = 30000;
const MAX_TIMEOUT_MS = 120000;

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
            "How long to block before returning an empty result (default 30000, max 120000).",
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

      // Keep presence fresh up front: a long block shouldn't let this agent's
      // TTL-backed presence lapse while it sits idle waiting.
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

      return jsonResult({
        timed_out,
        elapsed_ms,
        room_messages: roomMessages,
        direct_messages: directMessages,
      });
    },
  );
}
