// MCP tools for room membership: join_room, leave_room, list_rooms.
import { z } from "zod";
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import {
  getIdentity,
  getRooms,
  addRoom,
  removeRoom,
  syncPresence,
} from "../identity.js";
import {
  assertValidToken,
  ensureRoomConsumer,
  deleteRoomConsumer,
  listPresence,
} from "../stream-manager.js";
import { jsonResult } from "./register.js";

export function registerRoomTools(server: McpServer): void {
  server.registerTool(
    "join_room",
    {
      title: "Join Room",
      description: "Join a named room for multi-agent coordination",
      inputSchema: {
        room: z.string().describe("Room name to join"),
      },
    },
    async ({ room }) => {
      assertValidToken("room name", room);
      addRoom(room);
      await ensureRoomConsumer(getIdentity().id, room);
      await syncPresence();
      return jsonResult({ joined: true, room, rooms: getRooms() });
    },
  );

  server.registerTool(
    "leave_room",
    {
      title: "Leave Room",
      description: "Leave a room",
      inputSchema: {
        room: z.string().describe("Room name to leave"),
      },
    },
    async ({ room }) => {
      removeRoom(room);
      await deleteRoomConsumer(getIdentity().id, room);
      await syncPresence();
      return jsonResult({ left: true, room, rooms: getRooms() });
    },
  );

  server.registerTool(
    "list_rooms",
    {
      title: "List Rooms",
      description: "List all active rooms and their members",
      inputSchema: {},
    },
    async () => {
      const agents = await listPresence();

      const roomMap = new Map<string, Array<{ id: string; name: string }>>();

      for (const agent of agents) {
        for (const room of agent.rooms) {
          if (!roomMap.has(room)) {
            roomMap.set(room, []);
          }
          roomMap.get(room)!.push({ id: agent.id, name: agent.name });
        }
      }

      const rooms = Array.from(roomMap.entries())
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([room, members]) => ({
          room,
          members,
          member_count: members.length,
        }));

      return jsonResult({ rooms });
    },
  );
}
