import type { components } from "./schema";

export type SessionContext = components["schemas"]["SessionContext"];
export type Overview = components["schemas"]["Overview"];
export type Configuration = components["schemas"]["modelcairn-config-v1alpha1.schema"];
export type ConfigurationPlan = components["schemas"]["Plan"];
export type Resource = components["schemas"]["Resource"];
export type SecretMetadata = components["schemas"]["SecretMetadata"];
export type OperationalRequest = components["schemas"]["OperationalRequest"];
export type OperationalAttempt = components["schemas"]["OperationalAttempt"];
export type SettingsSpec = components["schemas"]["spec"] & { publicOrigin: string };
export type SettingsDocument = components["schemas"]["document"] & { resourceVersion: number; spec: SettingsSpec };
export type SettingsState = { desired: SettingsDocument; effective: SettingsDocument; restartRequired: boolean };
export type SettingsPlan = Omit<components["schemas"]["AdminSettingsPlan"], "desired"> & { desired: SettingsDocument };
export type Readiness = {
  status: "ready" | "not_ready";
  components: Record<string, { Ready: boolean; Reason: string }>;
};

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

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function isFiniteCount(value: unknown): value is number {
  return typeof value === "number" && Number.isFinite(value) && value >= 0;
}

function isOverview(value: unknown): value is Overview {
  if (!isRecord(value) || !isRecord(value.resourceCounts) || !isRecord(value.requests24h) || !Array.isArray(value.recentRequests)) return false;
  const resourceCounts = value.resourceCounts;
  const requests24h = value.requests24h;
  if (!Object.values(resourceCounts).every(isFiniteCount)) return false;
  if (!["total", "success", "error"].every((key) => isFiniteCount(requests24h[key]))) return false;
  if (!isFiniteCount(value.activeCooldowns) || typeof value.generatedAt !== "string") return false;
  const outcomes = new Set(["success", "error", "partial", "cancelled", "indeterminate"]);
  return value.recentRequests.every((item) => isRecord(item)
    && typeof item.id === "string"
    && typeof item.requestedAlias === "string"
    && typeof item.startedAt === "string"
    && (item.completedAt === undefined || item.completedAt === null || typeof item.completedAt === "string")
    && (item.outcome === undefined || item.outcome === null || (typeof item.outcome === "string" && outcomes.has(item.outcome)))
    && (item.httpStatus === undefined || item.httpStatus === null || isFiniteCount(item.httpStatus))
    && (item.durationMs === undefined || item.durationMs === null || isFiniteCount(item.durationMs))
    && isFiniteCount(item.attempts));
}

export const api = {
  async readiness(): Promise<Readiness> {
    const response = await fetch("/readyz", { headers: { Accept: "application/json" }, credentials: "same-origin" });
    const body = await response.json() as Readiness;
    if (body.status !== "ready" && body.status !== "not_ready") throw new APIError(response.status, "invalid_readiness");
    return body;
  },
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
  async overview() {
    const body = await request<unknown>("/overview");
    if (!isOverview(body)) throw new APIError(502, "invalid_overview");
    return body;
  },
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
  agentTokenStatus(name: string) { return request<{ tokenStatus: components["schemas"]["AgentTokenStatus"] }>(`/agent-tokens/${encodeURIComponent(name)}/status`); },
  revokeAgentToken(name: string) { return request<void>(`/agent-tokens/${encodeURIComponent(name)}/revoke`, { method: "POST" }); },
  listResources(kind: string) { return request<components["schemas"]["ResourcePage"]>(`/resources/${encodeURIComponent(kind)}?limit=200`); },
  createResource(kind: string, resource: Resource) { return request<Resource>(`/resources/${encodeURIComponent(kind)}`, { method: "POST", body: JSON.stringify(resource) }); },
  updateResource(kind: string, name: string, version: number, resource: Resource) { return request<Resource>(`/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`, { method: "PUT", headers: { "If-Match": `"${version}"` }, body: JSON.stringify(resource) }); },
  publishStrategy(name: string, version: number) { return request<Resource>(`/strategies/${encodeURIComponent(name)}/publish`, { method: "POST", headers: { "If-Match": `"${version}"` } }); },
  deleteResource(kind: string, name: string, version: number) { return request<void>(`/resources/${encodeURIComponent(kind)}/${encodeURIComponent(name)}`, { method: "DELETE", headers: { "If-Match": `"${version}"` } }); },
  listSecrets() { return request<{ items: SecretMetadata[]; nextCursor?: string | null }>("/secrets?limit=200"); },
  deleteSecret(name: string, version: number) { return request<void>(`/secrets/${encodeURIComponent(name)}`, { method: "DELETE", headers: { "If-Match": `"${version}"` } }); },
  listRequests(filters: { outcome?: string; alias?: string; from?: string; to?: string; cursor?: string }) {
    const query = new URLSearchParams({ limit: "50" });
    for (const [key,value] of Object.entries(filters)) if (value) query.set(key,value);
    return request<components["schemas"]["OperationalRequestPage"]>(`/requests?${query}`);
  },
  listAttempts(id: string) { return request<{ items: OperationalAttempt[] }>(`/requests/${encodeURIComponent(id)}/attempts`); },
  getSettings() { return request<SettingsState>("/settings"); },
  planSettings(document: SettingsDocument) { return request<SettingsPlan>("/settings/plan", { method: "POST", body: JSON.stringify(document) }); },
  applySettings(document: SettingsDocument, planToken: string) { return request<components["schemas"]["AdminSettingsApplyResult"]>("/settings/apply", { method: "POST", headers: { "X-ModelCairn-Plan-Token": planToken }, body: JSON.stringify(document) }); },
};
