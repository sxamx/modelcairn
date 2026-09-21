import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import Catalog from "./Catalog";

const resources: Record<string, unknown[]> = {
  providers: [{ kind: "Provider", metadata: { name: "google", displayName: "Google AI Studio" }, spec: {} }],
  "provider-accounts": [{ kind: "ProviderAccount", metadata: { name: "google-account" }, spec: { providerRef: { name: "google" } } }],
  "provider-connections": [{ kind: "ProviderConnection", metadata: { name: "google-api" }, spec: { providerRef: { name: "google" }, baseUrl: "https://example.invalid/v1" } }],
  credentials: [{ kind: "Credential", metadata: { name: "google-key" }, spec: { providerAccountRef: { name: "google-account" }, secretRef: { name: "secret" }, egressRef: { name: "direct" } } }],
  egresses: [{ kind: "Egress", metadata: { name: "direct" }, spec: { type: "direct" } }],
  models: [{ kind: "Model", metadata: { name: "google-model" }, spec: { connectionRef: { name: "google-api" }, providerModelId: "gemini-test", capabilities: ["text", "stream"] } }],
  destinations: [{ kind: "Destination", metadata: { name: "primary" }, spec: { modelRef: { name: "google-model" }, credentialRef: { name: "google-key" } } }],
  strategies: [{ kind: "Strategy", metadata: { name: "sequential" }, spec: { destinations: [{ name: "primary" }], maxAttempts: 1 } }],
  routes: [{ kind: "Route", metadata: { name: "assistant" }, spec: { modelAlias: "assistant", strategyRef: { name: "sequential" } } }],
};

afterEach(() => { vi.restoreAllMocks(); window.location.hash = ""; });

function mockResources(createStatus?: number) {
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    if (init?.method === "POST") return new Response(JSON.stringify({ error: { code: "already_exists" } }), {
      status: createStatus ?? 200, headers: { "Content-Type": "application/json" },
    });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), {
      status: 200, headers: { "Content-Type": "application/json" },
    });
  });
}

test("groups provider models and keys without exposing internal resources as primary tabs", async () => {
  window.location.hash = "#/providers"; mockResources();
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  expect(await screen.findByText("Google AI Studio")).toBeInTheDocument();
  expect(screen.getByText("1 clave")).toBeInTheDocument();
  expect(screen.getByText("1 modelo")).toBeInTheDocument();
  expect(screen.queryByRole("tab", { name: "Egresos" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByRole("link", { name: /Google AI Studio/ }));
  expect(await screen.findByRole("tab", { name: "API keys" })).toBeInTheDocument();
  expect(screen.getByText("Disponibilidad").closest("article")).toHaveTextContent("Sin datos");
});

test("keeps provider creation errors inside the visible dialog", async () => {
  window.location.hash = "#/providers"; mockResources(409);
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Añadir proveedor" }));
  fireEvent.change(screen.getByLabelText("Identificador técnico"), { target: { value: "google" } });
  fireEvent.click(screen.getByRole("button", { name: "Crear proveedor" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Ese identificador ya existe");
  expect(screen.getByRole("dialog")).toBeInTheDocument();
});

test("shows the configured route fallback chain using linked resources", async () => {
  window.location.hash = "#/routes"; mockResources();
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  expect(await screen.findByRole("link", { name: "assistant" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("link", { name: "assistant" }));
  expect(await screen.findByText("Clave: google-key")).toBeInTheDocument();
});
