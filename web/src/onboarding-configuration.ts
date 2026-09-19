import type { Configuration } from "./api/client";

export type OnboardingCapabilities = { stream: boolean; tools: boolean };

export function buildOnboardingConfiguration(
  name: string,
  displayName: string,
  baseUrl: string,
  modelID: string,
  secretName: string,
  alias: string,
  agentName: string,
  selected: OnboardingCapabilities = { stream: false, tools: false },
): Configuration {
  const ref = (value: string) => ({ name: value });
  const resource = (kind: string, suffix: string, spec: object, label?: string) => ({ kind, state: "present", metadata: { name: suffix ? `${name}-${suffix}` : name, ...(label ? { displayName: label } : {}) }, spec });
  const capabilities = ["text", ...(selected.stream ? ["stream"] : []), ...(selected.tools ? ["tools"] : [])];
  return { apiVersion: "modelcairn.io/v1alpha1", kind: "Configuration", resources: [
    resource("Provider", "", {}, displayName),
    resource("ProviderAccount", "account", { providerRef: ref(name) }),
    resource("ProviderConnection", "api", { providerRef: ref(name), baseUrl, adapter: "openai-chat-v1", allowPrivateNetwork: false, enabled: true }),
    resource("Egress", "direct", { type: "direct", enabled: true }),
    resource("Credential", "key", { providerAccountRef: ref(`${name}-account`), egressRef: ref(`${name}-direct`), secretRef: ref(secretName), enabled: true }),
    resource("Model", "model", { connectionRef: ref(`${name}-api`), providerModelId: modelID, capabilities, enabled: true }),
    resource("Destination", "primary", { modelRef: ref(`${name}-model`), credentialRef: ref(`${name}-key`), enabled: true, weight: 100 }),
    resource("Strategy", "sequential", { destinations: [ref(`${name}-primary`)], maxAttempts: 1, attemptTimeoutMs: 60000, totalTimeoutMs: 120000 }),
    { kind: "Route", state: "present", metadata: { name: `${name}-route` }, spec: { modelAlias: alias, strategyRef: ref(`${name}-sequential`), enabled: true } },
    { kind: "AgentToken", state: "present", metadata: { name: agentName }, spec: { allowedRouteRefs: [ref(`${name}-route`)], expiresAt: null, enabled: true } },
  ] } as Configuration;
}
