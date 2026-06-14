#!/usr/bin/env node
// Entry point: connect to NATS, ensure JetStream infrastructure, then serve
// the MCP tool surface over stdio.
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { connectNats, closeNats, NATS_URL } from "./nats-client.js";
import { ensureInfrastructure } from "./stream-manager.js";
import { startPresenceHeartbeat } from "./heartbeat.js";
import { registerAllTools } from "./tools/register.js";

async function main(): Promise<void> {
  try {
    await connectNats();
    await ensureInfrastructure();
  } catch (err) {
    console.error(
      `[nats-chat] failed to connect to NATS at ${NATS_URL}:`,
      err instanceof Error ? err.message : err,
    );
    process.exit(1);
  }

  const server = new McpServer({
    name: "nats-chat",
    version: "0.1.0",
  });
  registerAllTools(server);

  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error(`[nats-chat] connected to ${NATS_URL}, serving on stdio`);

  // Keep this session's presence fresh for as long as the process lives, even
  // through long stretches of work that call no chat tools.
  const stopHeartbeat = startPresenceHeartbeat();

  const shutdown = async () => {
    stopHeartbeat();
    await closeNats();
    process.exit(0);
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

try {
  await main();
} catch (err) {
  console.error("[nats-chat] fatal:", err);
  process.exit(1);
}
