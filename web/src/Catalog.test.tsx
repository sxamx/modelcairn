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
  expect(screen.getByText("Uptime").closest("article")).toHaveTextContent("Sin sondeos");
});

test("charts only observed model requests and never calls them uptime", async () => {
  window.location.hash = "#/providers/google/resumen";
  vi.spyOn(globalThis, "fetch").mockImplementation(async input => {
    const url = String(input);
    const body = url.endsWith("/metrics")
      ? { models: [{ name: "google-model", requests: 7, inputTokens: 1400, outputTokens: 300 }] }
      : { items: resources[/\/resources\/([^?]+)/.exec(url)?.[1] ?? ""] ?? [], nextCursor: null };
    return new Response(JSON.stringify(body), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  expect(await screen.findByRole("heading", { name: "Solicitudes por modelo" })).toBeInTheDocument();
  expect(await screen.findByText("7 solicitudes")).toBeInTheDocument();
  expect(screen.getByText("Uptime").closest("article")).toHaveTextContent("Sin sondeos");
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
  const link = (await screen.findByText("assistant")).closest("a");
  expect(link).not.toBeNull();
  fireEvent.click(link!);
  expect(await screen.findByText("DESTINO PRINCIPAL")).toBeInTheDocument();
  expect(screen.getByRole("button", { name: "Editar en el lienzo" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Configurar google-model" }));
  expect(await screen.findByRole("dialog")).toHaveTextContent("Salida de red");
  expect(screen.getByRole("dialog")).toHaveTextContent("google-key");
});

test("model detail returns to the exact provider and tab that opened it", async () => {
  window.location.hash = "#/providers/google/modelos"; mockResources();
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  const link = await screen.findByRole("link", { name: "google-model" });
  expect(link.getAttribute("href")).toContain("from=%23%2Fproviders%2Fgoogle%2Fmodelos");
  window.location.hash = link.getAttribute("href") ?? "";
  // The app switches catalog areas on navigation; mount that area as it does.
  render(<Catalog area="models" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  expect((await screen.findByRole("link", { name: /Volver al proveedor/ })).getAttribute("href")).toBe("#/providers/google/modelos");
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

test("adds an API connection from its provider", async () => {
  window.location.hash = "#/providers/google/configuracion";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (init?.method === "POST") return new Response(JSON.stringify({ kind: "ProviderConnection", metadata: { name: "google-second" }, spec: {} }), { status: 201, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: /Añadir conexión/ }));
  fireEvent.change(screen.getByLabelText("Identificador interno"), { target: { value: "google-second" } });
  fireEvent.change(screen.getByLabelText("URL base de la API"), { target: { value: "https://example.invalid/v1" } });
  fireEvent.click(screen.getByRole("button", { name: "Guardar conexión" }));
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/resources/provider-connections") && call.init?.method === "POST")).toBe(true));
  const saved = calls.find(call => call.url.endsWith("/resources/provider-connections") && call.init?.method === "POST");
  expect(JSON.parse(String(saved?.init?.body)).spec).toMatchObject({ providerRef: { name: "google" }, adapter: "openai-chat-v1", allowPrivateNetwork: false });
});

test("reviews a stored key link before applying", async () => {
  window.location.hash = "#/providers/google/claves";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input); calls.push({ url, init });
    if (url.includes("/config/plan")) return new Response(JSON.stringify({ planToken: "review-token", changes: [{ kind: "Credential", name: "google-second-key", operation: "create" }] }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (url.includes("/config/apply")) return new Response(JSON.stringify({ applied: true }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (url.includes("/secrets?")) return new Response(JSON.stringify({ items: [{ name: "secret" }], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(url)?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="providers" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: /Vincular API key/ }));
  fireEvent.change(await screen.findByLabelText("Clave guardada"), { target: { value: "secret" } });
  fireEvent.change(screen.getByLabelText("Identificador del vínculo"), { target: { value: "google-second-key" } });
  fireEvent.click(screen.getByRole("button", { name: "Revisar vínculo" }));
  expect(await screen.findByText(/API key vinculada · google-second-key/)).toBeInTheDocument();
  expect(calls.some(call => call.url.includes("/config/apply"))).toBe(false);
  const planned = calls.find(call => call.url.includes("/config/plan"));
  expect(JSON.parse(String(planned?.init?.body)).resources).toEqual([expect.objectContaining({ kind: "Credential", spec: expect.objectContaining({ providerAccountRef: { name: "google-account" }, secretRef: { name: "secret" }, egressRef: { name: "direct" } }) })]);
  fireEvent.click(screen.getByRole("button", { name: "Confirmar vínculo" }));
  await waitFor(() => expect(calls.some(call => call.url.includes("/config/apply"))).toBe(true));
});

test("saving a visual route remains a draft until explicitly published", async () => {
  window.location.hash = "#/routes/assistant";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "confirm").mockReturnValue(true);
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (String(input).endsWith("/config/plan")) return new Response(JSON.stringify({ planToken: "plan-token", changes: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (String(input).endsWith("/config/apply")) return new Response(JSON.stringify({ applied: true }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (String(input).endsWith("/strategies/sequential/publish")) return new Response(JSON.stringify({ kind: "Strategy", metadata: { name: "sequential", resourceVersion: 2 }, spec: {} }), { status: 200, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Editar en el lienzo" }));
  fireEvent.click(screen.getAllByRole("button", { name: "Guardar borrador" })[0]);
  await screen.findByText(/Recorrido guardado como borrador/);
  expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish"))).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Publicar borrador guardado" }));
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish") && call.init?.method === "POST")).toBe(true));
});

test("adds a compatible node locally, then saves it atomically as a draft", async () => {
  window.location.hash = "#/routes/assistant";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.stubGlobal("crypto", { randomUUID: () => "12345678-aaaa-4aaa-8aaa-aaaaaaaaaaaa" });
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    calls.push({ url: String(input), init });
    if (String(input).endsWith("/config/plan")) return new Response(JSON.stringify({ planToken: "draft-plan", changes: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (String(input).endsWith("/config/apply")) return new Response(JSON.stringify({ applied: true }), { status: 200, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(String(input))?.[1] ?? "";
    return new Response(JSON.stringify({ items: resources[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Editar en el lienzo" }));
  fireEvent.click(screen.getByRole("button", { name: "Añadir modelo google-model" }));
  expect(screen.getByLabelText("Clave API")).toHaveValue("google-key");
  expect(calls.some(call => call.url.endsWith("/config/apply"))).toBe(false);
  fireEvent.click(screen.getByRole("button", { name: "Listo" }));
  fireEvent.click(screen.getAllByRole("button", { name: "Guardar borrador" })[0]);
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/config/apply"))).toBe(true));
  expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish"))).toBe(false);
  const planned = calls.find(call => call.url.endsWith("/config/plan"));
  expect(JSON.parse(String(planned?.init?.body)).resources).toEqual([
    expect.objectContaining({ kind: "Destination", spec: expect.objectContaining({ modelRef: { name: "google-model" }, credentialRef: { name: "google-key" } }) }),
    expect.objectContaining({ kind: "Strategy", spec: expect.objectContaining({ maxAttempts: 2 }) }),
  ]);
});

test("uses the visual node order as the saved fallback order", async () => {
  window.location.hash = "#/routes/assistant";
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const configured = {
    ...resources,
    destinations: [
      ...resources.destinations,
      { kind: "Destination", metadata: { name: "backup" }, spec: { modelRef: { name: "google-model" }, credentialRef: { name: "google-key" } } },
    ],
    strategies: [{ kind: "Strategy", metadata: { name: "sequential", resourceVersion: 1 }, spec: { destinations: [{ name: "primary" }, { name: "backup" }], maxAttempts: 2 } }],
  };
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input, init) => {
    const url = String(input); calls.push({ url, init });
    if (url.endsWith("/config/plan")) return new Response(JSON.stringify({ planToken: "reorder-plan", changes: [] }), { status: 200, headers: { "Content-Type": "application/json" } });
    if (url.endsWith("/config/apply")) return new Response(JSON.stringify({ applied: true }), { status: 200, headers: { "Content-Type": "application/json" } });
    const kind = /\/resources\/([^?]+)/.exec(url)?.[1] ?? "";
    return new Response(JSON.stringify({ items: (configured as Record<string, unknown[]>)[kind] ?? [], nextCursor: null }), { status: 200, headers: { "Content-Type": "application/json" } });
  });
  render(<Catalog area="routes" onOpenWizard={() => undefined} onOpenSecrets={() => undefined}/>);
  fireEvent.click(await screen.findByRole("button", { name: "Editar en el lienzo" }));
  fireEvent.click(screen.getAllByRole("button", { name: "Configurar google-model" })[1]);
  fireEvent.click(screen.getByRole("button", { name: "↑ Subir" }));
  fireEvent.click(screen.getByRole("button", { name: "Listo" }));
  fireEvent.click(screen.getByRole("button", { name: "Guardar borrador" }));
  await waitFor(() => expect(calls.some(call => call.url.endsWith("/config/plan"))).toBe(true));
  const plan = calls.find(call => call.url.endsWith("/config/plan"));
  expect(JSON.parse(String(plan?.init?.body)).resources[0].spec.destinations).toEqual([{ name: "backup" }, { name: "primary" }]);
  expect(calls.some(call => call.url.endsWith("/strategies/sequential/publish"))).toBe(false);
});
