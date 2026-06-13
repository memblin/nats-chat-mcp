export interface AgentIdentity {
  id: string;
  name: string;
  role: string;
}

export interface AgentPresence extends AgentIdentity {
  rooms: string[];
  last_seen: string;
}

export interface Message {
  id: string;
  from: string;
  from_id: string;
  role: string;
  room?: string;
  content: string;
  timestamp: string;
  reply_to?: string;
}
