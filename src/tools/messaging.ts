// MCP tools for room messaging: send_message, check_messages, get_history.
import type { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import {
  getIdentity,
  getRooms,
  hasRoom,
  newMessage,
  syncPresence,
} from "../identity.js";
import {
  assertValidToken,
  publishRoomMessage,
  fetchRoomMessages,
  getRoomHistory,
} from "../stream-manager.js";
import { resetEmptyWakeups } from "../wakeups.js";
import { jsonResult } from "./register.js";

export function registerMessagingTools(server: McpServer): void {
  server.registerTool(
    "send_message",
    {
      title: "Send Message",
      description: "Broadcast a message to a room",
      inputSchema: {
        room: z.string().describe("The room to send the message to"),
        content: z.string().describe("The message content to broadcast"),
        reply_to: z
          .string()
          .optional()
          .describe("Optional message ID this is a reply to"),
      },
    },
    async ({ room, content, reply_to }) => {
      assertValidToken("room name", room);
      if (!hasRoom(room)) {
        throw new Error(
          `You are not a member of room "${room}". Use join_room first.`,
        );
      }
      const message = newMessage(content, { room, reply_to });
      await publishRoomMessage(room, message);
      await syncPresence();
      return jsonResult({ sent: true, message });
    },
  );

  server.registerTool(
    "check_messages",
    {
      title: "Check Messages",
      description: "Poll for new messages in joined rooms",
      inputSchema: {
        room: z
          .string()
          .optional()
          .describe("Specific room to check; omit to check all joined rooms"),
      },
    },
    async ({ room }) => {
      const identity = getIdentity();

      if (room !== undefined) {
        if (!hasRoom(room)) {
          throw new Error(
            `You are not a member of room "${room}". Use join_room first.`,
          );
        }
        const messages = await fetchRoomMessages(identity.id, room);
        if (messages.length > 0) resetEmptyWakeups(identity.id);
        await syncPresence();
        return jsonResult({
          rooms_checked: [room],
          messages,
          count: messages.length,
        });
      }

      const rooms = getRooms();
      if (rooms.length === 0) {
        await syncPresence();
        return jsonResult({
          rooms_checked: [],
          messages: [],
          count: 0,
          hint: "You have not joined any rooms. Use join_room to join a room first.",
        });
      }

      const allMessages = [];
      for (const r of rooms) {
        const msgs = await fetchRoomMessages(identity.id, r);
        allMessages.push(...msgs);
      }
      if (allMessages.length > 0) resetEmptyWakeups(identity.id);

      await syncPresence();
      return jsonResult({
        rooms_checked: rooms,
        messages: allMessages,
        count: allMessages.length,
      });
    },
  );

  server.registerTool(
    "get_history",
    {
      title: "Get History",
      description: "Retrieve message history for a room",
      inputSchema: {
        room: z.string().describe("The room to retrieve history for"),
        limit: z
          .number()
          .int()
          .positive()
          .optional()
          .describe("Maximum number of messages to return (default: 50)"),
      },
    },
    async (input) => {
      const { room } = input;
      assertValidToken("room name", room);
      const limit = input.limit ?? 50;
      const messages = await getRoomHistory(room, limit);
      return jsonResult({ room, count: messages.length, messages });
    },
  );
}
