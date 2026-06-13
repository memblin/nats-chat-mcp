#!/usr/bin/env node
// Entry point: connect to NATS, ensure JetStream infrastructure, then serve
// the MCP tool surface over stdio.
import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { connectNats, closeNats, NATS_URL } from "./nats-client.js";
import { ensureInfrastructure } from "./stream-manager.js";
import { registerAllTools } from "./tools/register.js";

async function main(): Promise<void> {
  try {
    await connectNats();
    await ensureInfrastructure();
  } catch (err) {
    console.error(
      `[claude-connect-nats-mcp] failed to connect to NATS at ${NATS_URL}:`,
      err instanceof Error ? err.message : err,
    );
    process.exit(1);
  }

  const server = new McpServer({
    name: "claude-connect-nats-mcp",
    version: "0.1.0",
  });
  registerAllTools(server);

  const transport = new StdioServerTransport();
  await server.connect(transport);
  console.error(
    `[claude-connect-nats-mcp] connected to ${NATS_URL}, serving on stdio`,
  );

  const shutdown = async () => {
    await closeNats();
    process.exit(0);
  };
  process.on("SIGINT", shutdown);
  process.on("SIGTERM", shutdown);
}

try {
  await main();
} catch (err) {
  console.error("[claude-connect-nats-mcp] fatal:", err);
  process.exit(1);
}
