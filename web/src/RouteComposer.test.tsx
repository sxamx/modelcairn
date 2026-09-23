import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import RouteComposer from "./RouteComposer";
import { Resource } from "./api/client";

const resource = (kind: string, id: string, spec: object): Resource =>
  ({ kind, metadata: { name: id }, spec }) as Resource;
const providers = [resource("Provider", "google", {}), resource("Provider", "openrouter", {})];
const connections = [
  resource("ProviderConnection", "google-api", { providerRef: { name: "google" } }),
  resource("ProviderConnection", "openrouter-api", { providerRef: { name: "openrouter" } }),
];
const accounts = [
  resource("ProviderAccount", "google-account", { providerRef: { name: "google" } }),
  resource("ProviderAccount", "openrouter-account", { providerRef: { name: "openrouter" } }),
];
const models = [
  resource("Model", "gemini", { connectionRef: { name: "google-api" }, enabled: true }),
  resource("Model", "oss", { connectionRef: { name: "openrouter-api" }, enabled: true }),
];
const credentials = [
  resource("Credential", "google-key", { providerAccountRef: { name: "google-account" }, enabled: true }),
  resource("Credential", "openrouter-key", { providerAccountRef: { name: "openrouter-account" }, enabled: true }),
];

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); });

test("plans a compatible fallback route before creating it atomically", async () => {
  vi.stubGlobal("crypto", { randomUUID: () => "12345678-aaaa-4aaa-8aaa-aaaaaaaaaaaa" });
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    const body = String(input).endsWith("/config/plan")
      ? { planToken: "plan-token", changes: [{ kind: "Route", name: "route-12345678-aaa", operation: "create" }] }
      : { applied: true };
    return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  const created = vi.fn();
  render(<RouteComposer models={models} credentials={credentials} connections={connections} accounts={accounts} providers={providers} onCancel={() => undefined} onCreated={created}/>);
  fireEvent.change(screen.getByLabelText("Alias para tus aplicaciones"), { target: { value: "assistant" } });
  fireEvent.change(screen.getByLabelText("Modelo"), { target: { value: "gemini" } });
  expect(within(screen.getByLabelText("Clave API")).queryByRole("option", { name: "openrouter-key" })).toBeNull();
  fireEvent.change(screen.getByLabelText("Clave API"), { target: { value: "google-key" } });
  fireEvent.click(screen.getByRole("button", { name: "+ Añadir respaldo" }));
  fireEvent.change(screen.getByLabelText("Modelo"), { target: { value: "oss" } });
  expect(within(screen.getByLabelText("Clave API")).queryByRole("option", { name: "google-key" })).toBeNull();
  fireEvent.change(screen.getByLabelText("Clave API"), { target: { value: "openrouter-key" } });
  fireEvent.click(screen.getByRole("button", { name: "Revisar ruta" }));
  await screen.findByRole("region", { name: "Revisión de la ruta" });
  expect(calls).toHaveLength(1);
  const planned = JSON.parse(String(calls[0].init?.body));
  expect(planned.resources.map((item: { kind: string }) => item.kind)).toEqual(["Destination", "Destination", "Strategy", "Route"]);
  expect(planned.resources[2].spec.maxAttempts).toBe(2);
  expect(planned.resources[3].spec.modelAlias).toBe("assistant");
  fireEvent.click(screen.getByRole("button", { name: "Crear y activar ruta" }));
  await waitFor(() => expect(created).toHaveBeenCalledWith("route-12345678-aaa"));
  expect(calls[1].url).toContain("/config/apply");
  expect(new Headers(calls[1].init?.headers).get("X-ModelCairn-Plan-Token")).toBe("plan-token");
  expect(JSON.parse(String(calls[1].init?.body))).toEqual(planned);
});
