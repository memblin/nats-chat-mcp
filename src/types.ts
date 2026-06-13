// Shared data shapes that flow through the system: agent identities, the
// presence records stored in the KV registry, and the messages published to
// NATS subjects. Every JSON payload on the wire conforms to one of these.

export interface AgentIdentity {
  id: string;
  name: string;
}

export interface AgentPresence extends AgentIdentity {
  rooms: string[];
  last_seen: string;
}

export interface Message {
  id: string;
  from: string;
  from_id: string;
  room?: string;
  content: string;
  timestamp: string;
  reply_to?: string;
}
