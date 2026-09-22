import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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
  strategies: [{ kind: "Strategy", metadata: { name: "sequential", resourceVersion: 1 }, spec: { destinations: [{ name: "primary" }], maxAttempts: 1 } }],
  routes: [{ kind: "Route", metadata: { name: "assistant" }, spec: { modelAlias: "assistant", strategyRef: { name: "sequential" } } }],
};

afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals(); window.location.hash = ""; });

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
  expect(await screen.findByText(/Clave google-key/)).toBeInTheDocument();
  expect(screen.getByText("PRIMERA OPCIÓN")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Editar recorrido" })).toBeInTheDocument();
});

test("model detail returns to the exact provider and tab that opened it", async () => {
  window.location.hash = "#/providers/google/modelos"; mockResources();
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  const link = await screen.findByRole("link", { name: "google-model" });
  expect(link.getAttribute("href")).toContain("from=%23%2Fproviders%2Fgoogle%2Fmodelos");
  window.location.hash = link.getAttribute("href") ?? "";
  // The app switches catalog areas on navigation; mount that area as it does.
  render(<Catalog area="models" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  expect((await screen.findByRole("link", { name: /Volver a modelos/ })).getAttribute("href")).toBe("#/providers/google/modelos");
});

test("adds a model from within its provider without writing a key", async () => {
  window.location.hash = "#/providers/google/modelos";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (init?.method === "POST") return new Response(JSON.stringify({ kind: "Model", metadata: { name: "second" }, spec: {} }), { status: 201, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: /Añadir modelo/ }));
  fireEvent.change(screen.getByLabelText("Conexión API"), { target: { value: "google-api" } });
  fireEvent.change(screen.getByLabelText("Identificador interno"), { target: { value: "second" } });
  fireEvent.change(screen.getByLabelText("ID exacto del modelo en el proveedor"), { target: { value: "gemini-second" } });
  fireEvent.click(screen.getByRole("button", { name: "Guardar modelo" }));
  await waitFor(() => expect(calls.some(call => call.init?.method === "POST" && call.url.endsWith("/resources/models"))).toBe(true));
  const saved = calls.find(call => call.init?.method === "POST");
  expect(JSON.parse(String(saved?.init?.body)).spec).toMatchObject({ connectionRef: { name: "google-api" }, providerModelId: "gemini-second", capabilities: ["text"] });
});

test("saving a visual route remains a draft until explicitly published", async () => {
  window.location.hash = "#/routes/assistant";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "confirm").mockReturnValue(true);
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (init?.method) return new Response(JSON.stringify({ kind: "Strategy", metadata: { name: "sequential", resourceVersion: 2 }, spec: {} }), { status: 200, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Editar recorrido" }));
  fireEvent.click(screen.getByRole("button", { name: "Guardar borrador" }));
  await screen.findByText(/Recorrido guardado como borrador/);
  expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish"))).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Publicar versión guardada" }));
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish") && call.init?.method === "POST")).toBe(true));
});

test("creates a route option from compatible model and key without publishing it", async () => {
  window.location.hash = "#/routes/assistant";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.stubGlobal("crypto", { randomUUID: () => "12345678-aaaa-4aaa-8aaa-aaaaaaaaaaaa" });
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (init?.method === "POST") return new Response(JSON.stringify({ kind: "Destination", metadata: { name: "option-12345678" }, spec: {} }), { status: 201, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Editar recorrido" }));
  fireEvent.change(screen.getByLabelText("Modelo"), { target: { value: "google-model" } });
  fireEvent.change(screen.getByLabelText("API key vinculada"), { target: { value: "google-key" } });
  fireEvent.click(screen.getByRole("button", { name: "Crear y añadir opción" }));
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/resources/destinations") && call.init?.method === "POST")).toBe(true));
  expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish"))).toBe(false);
  const destination = calls.find(call => call.url.endsWith("/resources/destinations") && call.init?.method === "POST");
  expect(JSON.parse(String(destination?.init?.body)).spec).toMatchObject({ modelRef: { name: "google-model" }, credentialRef: { name: "google-key" } });
});
