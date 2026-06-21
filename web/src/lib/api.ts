// Typed client for the Cylon REST API. Every UI action maps to an /api endpoint.

export interface Gateway {
  eui: string;
  region: string;
  sub_band: number;
  connection_mode: string;
  status: string;
  tcp_addr?: string;
  tag_conns: number;
  ws_clients: number;
}

export interface TagSession {
  joined: boolean;
  dev_addr?: string;
  fcnt_up: number;
  fcnt_down: number;
  dev_nonce: number;
}

export interface Tag {
  id: number;
  dev_eui: string;
  join_eui: string;
  app_key_masked: string;
  class: string;
  region: string;
  sub_band: number;
  default_dr: number;
  fport: number;
  payload_type: string;
  enabled: boolean;
  running: boolean;
  created_at: string;
  updated_at: string;
  session?: TagSession;
}

export interface Event {
  id: number;
  ts: string;
  tag_id?: number;
  direction: string;
  kind: string;
  freq?: number;
  dr?: number;
  fcnt?: number;
  fport?: number;
  payload_hex?: string;
  decoded?: string;
  result?: string;
}

async function req<T>(method: string, path: string, body?: unknown): Promise<T> {
  const res = await fetch(path, {
    method,
    headers: body ? { "Content-Type": "application/json" } : undefined,
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const j = await res.json();
      if (j.error) msg = j.error;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  if (res.status === 204) return undefined as T;
  return res.json() as Promise<T>;
}

export const api = {
  getGateway: () => req<Gateway>("GET", "/api/gateway"),
  updateGateway: (b: Partial<Gateway>) => req<Gateway>("PUT", "/api/gateway", b),
  listTags: () => req<Tag[]>("GET", "/api/tags"),
  getTag: (id: number) => req<Tag>("GET", `/api/tags/${id}`),
  createTags: (b: Record<string, unknown>) => req<Tag[]>("POST", "/api/tags", b),
  deleteTag: (id: number) => req<void>("DELETE", `/api/tags/${id}`),
  joinTag: (id: number) => req<{ status: string }>("POST", `/api/tags/${id}/join`),
  uplinkTag: (id: number, payloadHex?: string) =>
    req<{ status: string }>("POST", `/api/tags/${id}/uplink`, payloadHex ? { payload_hex: payloadHex } : {}),
  listEvents: (limit = 100) => req<Event[]>("GET", `/api/events?limit=${limit}`),
  runScenario: (name: string, b?: Record<string, unknown>) =>
    req<Record<string, unknown>>("POST", `/api/scenarios/${name}/run`, b ?? {}),
};
