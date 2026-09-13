import type { components } from "./schema";

export type SessionContext = components["schemas"]["SessionContext"];
export type Overview = components["schemas"]["Overview"];

type APIErrorBody = { error?: { code?: string; message?: string; requestId?: string } | string };

export class APIError extends Error {
  constructor(public readonly status: number, public readonly code: string, public readonly requestId?: string) {
    super(code);
  }
}

let csrfToken = "";

async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body) headers.set("Content-Type", "application/json");
  if (csrfToken && init.method && init.method !== "GET") headers.set("X-CSRF-Token", csrfToken);
  const response = await fetch(`/api/v1/admin${path}`, { ...init, headers, credentials: "same-origin" });
  if (!response.ok) {
    let body: APIErrorBody = {};
    try { body = await response.json() as APIErrorBody; } catch { /* uniform fallback */ }
    const detail = typeof body.error === "object" ? body.error : undefined;
    throw new APIError(response.status, detail?.code ?? (typeof body.error === "string" ? body.error : "request_failed"), detail?.requestId);
  }
  if (response.status === 204) return undefined as T;
  return response.json() as Promise<T>;
}

function rememberSession(session: SessionContext): SessionContext {
  csrfToken = session.csrfToken;
  return session;
}

export const api = {
  async login(username: string, password: string) {
    return rememberSession(await request<SessionContext>("/session", {
      method: "POST",
      body: JSON.stringify({ username, password }),
    }));
  },
  async session() {
    return rememberSession(await request<SessionContext>("/session/me"));
  },
  async logout() {
    try { await request<void>("/session", { method: "DELETE" }); } finally { csrfToken = ""; }
  },
  overview() { return request<Overview>("/overview"); },
};
