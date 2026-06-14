// MCP tools for point-to-point messaging: send_direct, send_ack, check_direct.
import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { getIdentity, newAck, newMessage, syncPresence } from "../identity.js";
import {
  publishDirectMessage,
  fetchDirectMessages,
  listPresence,
} from "../stream-manager.js";
import { resetEmptyWakeups } from "../wakeups.js";
import type { AckStatus, AgentPresence } from "../types.js";
import { jsonResult } from "./register.js";

// Ack statuses accepted by send_ack — kept in sync with AckStatus in types.ts.
const ACK_STATUSES: readonly [AckStatus, ...AckStatus[]] = [
  "received",
  "investigating",
  "dispatching",
  "in_progress",
  "blocked",
  "complete",
];

/**
 * Resolve a `to` argument (a name or an id) to a single registered agent. Names
 * can collide across agents; ids never do — so we accept an exact id match, or a
 * name match only when it is unambiguous. Shared by send_direct and send_ack so
 * both address recipients identically.
 */
async function resolveTarget(to: string): Promise<AgentPresence> {
  const agents = await listPresence();
  const matches = agents.filter((a) => a.id === to || a.name === to);

  if (matches.length === 0) {
    throw new Error(
      `No registered agent matches "${to}". Use list_agents to see available agents.`,
    );
  }
  if (matches.length > 1 && !matches.some((a) => a.id === to)) {
    const ids = matches.map((a) => a.id).join(", ");
    throw new Error(
      `Ambiguous agent name "${to}" — multiple agents share this name. Use one of these exact ids: ${ids}`,
    );
  }
  // Prefer an exact id match; otherwise the single unambiguous name match.
  return matches.find((a) => a.id === to) ?? matches[0];
}

export function registerDirectTools(server: McpServer): void {
  server.registerTool(
    "send_direct",
    {
      title: "Send Direct Message",
      description: "Send a direct message to another agent",
      inputSchema: {
        to: z.string().describe("The target agent's name or id"),
        content: z.string().describe("The message content to send"),
        reply_to: z
          .string()
          .optional()
          .describe("Optional message id this is a reply to"),
      },
    },
    async ({ to, content, reply_to }) => {
      const target = await resolveTarget(to);

      const message = newMessage(content, { reply_to });
      await publishDirectMessage(target.id, message);
      await syncPresence();

      return jsonResult({
        sent: true,
        to: { id: target.id, name: target.name },
        message,
      });
    },
  );

  server.registerTool(
    "send_ack",
    {
      title: "Send Acknowledgment",
      description:
        'Acknowledge a message you received and are acting on — a lightweight status ping to the sender, not a full reply. Call it immediately on wakeup, before doing work, so the sender gets delivery confirmation within seconds. Delivered on the same direct channel as send_direct but tagged type: "ack".',
      inputSchema: {
        to: z.string().describe("The target agent's name or id"),
        regarding: z
          .string()
          .describe(
            "Brief label for what is being acknowledged, e.g. 'rc13 publish handoff'",
          ),
        status: z
          .enum(ACK_STATUSES)
          .describe(
            "Current handling status: received | investigating | dispatching | in_progress | blocked | complete",
          ),
        note: z
          .string()
          .max(200)
          .optional()
          .describe("Optional human-readable detail (max 200 chars)"),
      },
    },
    async ({ to, regarding, status, note }) => {
      const target = await resolveTarget(to);

      const ack = newAck(regarding, status, note);
      await publishDirectMessage(target.id, ack);
      await syncPresence();

      return jsonResult({
        delivered: true,
        to: target.id,
        regarding,
        timestamp: ack.timestamp,
      });
    },
  );

  server.registerTool(
    "check_direct",
    {
      title: "Check Direct Messages",
      description: "Check for direct messages",
      inputSchema: {},
    },
    async () => {
      const messages = await fetchDirectMessages(getIdentity().id);
      if (messages.length > 0) resetEmptyWakeups(getIdentity().id);
      await syncPresence();

      return jsonResult({ count: messages.length, messages });
    },
  );
}
