import type { components } from "./schema";

export type SessionContext = components["schemas"]["SessionContext"];
export type Overview = components["schemas"]["Overview"];
export type Configuration = components["schemas"]["modelcairn-config-v1alpha1.schema"];
export type ConfigurationPlan = components["schemas"]["Plan"];
export type Resource = components["schemas"]["Resource"];
export type SecretMetadata = components["schemas"]["SecretMetadata"];

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
  putSecret(name: string, value: string, version?: number) {
    const headers = version === undefined ? undefined : { "If-Match": `"${version}"` };
    return request<components["schemas"]["SecretMetadata"]>(`/secrets/${encodeURIComponent(name)}`, { method: "PUT", headers, body: JSON.stringify({ value }) });
  },
  getSecret(name: string) { return request<components["schemas"]["SecretMetadata"]>(`/secrets/${encodeURIComponent(name)}`); },
  async saveSecret(name: string, value: string) {
    try { return await this.putSecret(name, value); }
    catch (error) {
      if (!(error instanceof APIError) || error.code !== "precondition_required") throw error;
      const metadata = await this.getSecret(name);
      return this.putSecret(name, value, metadata.resourceVersion);
    }
  },
  planConfiguration(configuration: Configuration) {
    return request<ConfigurationPlan>("/config/plan", { method: "POST", body: JSON.stringify(configuration) });
  },
  applyConfiguration(configuration: Configuration, planToken: string) {
    return request<components["schemas"]["ApplyResult"]>("/config/apply", { method: "POST", headers: { "X-ModelCairn-Plan-Token": planToken }, body: JSON.stringify(configuration) });
  },
  issueAgentToken(name: string) {
    return request<{ token: string; tokenStatus: components["schemas"]["AgentTokenStatus"] }>(`/agent-tokens/${encodeURIComponent(name)}/issue`, { method: "POST" });
  },
  listResources(kind: string) { return request<components["schemas"]["ResourcePage"]>(`/resources/${encodeURIComponent(kind)}?limit=200`); },
  deleteResource(kind: string, name: string, version: number) { return request<void>(`/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`, { method: "DELETE", headers: { "If-Match": `"${version}"` } }); },
  listSecrets() { return request<{ items: SecretMetadata[]; nextCursor?: string | null }>("/secrets?limit=200"); },
  deleteSecret(name: string, version: number) { return request<void>(`/secrets/${encodeURIComponent(name)}`, { method: "DELETE", headers: { "If-Match": `"${version}"` } }); },
};
