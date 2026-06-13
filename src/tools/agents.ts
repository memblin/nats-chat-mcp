// MCP tool for agent discovery: list_agents (reads the presence registry).
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { getIdentity, isRegistered, syncPresence } from "../identity.js";
import { listPresence } from "../stream-manager.js";
import { jsonResult } from "./register.js";

export function registerAgentsTools(server: McpServer): void {
  server.registerTool(
    "list_agents",
    {
      title: "List Agents",
      description: "List all registered agents and their presence",
      inputSchema: {},
    },
    async () => {
      if (isRegistered()) await syncPresence();

      const agents = await listPresence();
      const selfId = isRegistered() ? getIdentity().id : null;

      const sorted = [...agents].sort((a, b) => a.name.localeCompare(b.name));

      const mapped = sorted.map((agent) => ({
        id: agent.id,
        name: agent.name,
        rooms: agent.rooms,
        last_seen: agent.last_seen,
        is_self: agent.id === selfId,
      }));

      return jsonResult({ count: mapped.length, agents: mapped });
    },
  );
}
