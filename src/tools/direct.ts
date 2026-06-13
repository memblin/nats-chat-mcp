// MCP tools for point-to-point messaging: send_direct, check_direct.
import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { getIdentity, newMessage, syncPresence } from "../identity.js";
import {
  publishDirectMessage,
  fetchDirectMessages,
  listPresence,
} from "../stream-manager.js";
import { jsonResult } from "./register.js";

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
      // `to` may be a human-chosen name OR a unique id. Names can collide
      // across agents; ids never do — so we resolve against both and, below,
      // prefer an id match and only accept a name match when it's unambiguous.
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

      // Prefer exact id match if present, otherwise take the single name match
      const target = matches.find((a) => a.id === to) ?? matches[0];

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
    "check_direct",
    {
      title: "Check Direct Messages",
      description: "Check for direct messages",
      inputSchema: {},
    },
    async () => {
      const messages = await fetchDirectMessages(getIdentity().id);
      await syncPresence();

      return jsonResult({ count: messages.length, messages });
    },
  );
}
