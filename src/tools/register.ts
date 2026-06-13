// Aggregator: wires every tool group onto the MCP server. Also owns the two
// identity tools (register_agent / get_status) since they mutate session state.
import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import {
  getIdentity,
  getRooms,
  isRegistered,
  register,
  syncPresence,
} from "../identity.js";
import { assertValidToken, ensureDirectConsumer } from "../stream-manager.js";
import { NATS_URL } from "../nats-client.js";
import { registerRoomTools } from "./rooms.js";
import { registerMessagingTools } from "./messaging.js";
import { registerDirectTools } from "./direct.js";
import { registerAgentsTools } from "./agents.js";

/** Helper shared by tool modules: shape a JSON value as MCP text content. */
export function jsonResult(value: unknown) {
  return {
    content: [{ type: "text" as const, text: JSON.stringify(value, null, 2) }],
  };
}

export function registerAllTools(server: McpServer): void {
  registerIdentityTools(server);
  registerRoomTools(server);
  registerMessagingTools(server);
  registerDirectTools(server);
  registerAgentsTools(server);
}

function registerIdentityTools(server: McpServer): void {
  server.registerTool(
    "register_agent",
    {
      title: "Register Agent",
      description:
        "Register this Claude session as a named agent so other sessions can find and message it.",
      inputSchema: {
        name: z
          .string()
          .describe("Short agent name, e.g. 'build-seat-1' or 'lead'"),
      },
    },
    async ({ name }) => {
      assertValidToken("agent name", name);
      const identity = await register(name);
      // Create the DM consumer eagerly at registration time so a direct message
      // sent right after this agent registers isn't missed before its first
      // check_direct call (the consumer's cursor must exist to capture it).
      await ensureDirectConsumer(identity.id);
      return jsonResult({
        registered: true,
        identity,
        nats_url: NATS_URL,
      });
    },
  );

  server.registerTool(
    "get_status",
    {
      title: "Get Status",
      description:
        "Return this session's agent identity, joined rooms, and connection status.",
      inputSchema: {},
    },
    async () => {
      if (!isRegistered()) {
        return jsonResult({
          registered: false,
          nats_url: NATS_URL,
          hint: "Call register_agent to set this session's name.",
        });
      }
      await syncPresence();
      return jsonResult({
        registered: true,
        identity: getIdentity(),
        rooms: getRooms(),
        nats_url: NATS_URL,
      });
    },
  );
}
