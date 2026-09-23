import { fireEvent, render, screen, waitFor, within } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import App from "./App";

afterEach(() => {
  vi.restoreAllMocks();
  localStorage.clear();
  sessionStorage.clear();
  window.location.hash = "";
});

function response(status: number, body?: unknown) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  }));
}

test("recovers an existing administrative session", async () => {
  const calls: Array<{url:string; init?:RequestInit}> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
    calls.push({url:String(input),init});
    if (String(input).endsWith("/session/me")) return response(200, {
      admin: { id: "d79b24b4-4da4-43bf-829b-940e0f025d33", username: "sam" },
      csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z",
    });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    return response(200, { incompatible: true });
  });
  render(<App />);
  expect(await screen.findByRole("heading", { name: "Hola, sam" })).toBeInTheDocument();
  expect(await screen.findByText("El resumen no está disponible. Tus rutas siguen funcionando de forma independiente.")).toBeInTheDocument();
  expect(fetch).toHaveBeenCalledWith("/api/v1/admin/session/me", expect.objectContaining({ credentials: "same-origin" }));
  const protectedRead = calls.find((call) => call.url.endsWith("/overview"));
  expect(new Headers(protectedRead?.init?.headers).get("X-CSRF-Token")).toBe("csrf");
});

test("signs in without persisting the password", async () => {
  const fetchMock = vi.spyOn(globalThis, "fetch")
    .mockImplementationOnce(() => response(401, { error: { code: "authentication_required" } }))
    .mockImplementationOnce((_input, init) => {
      expect(init?.body).toBe(JSON.stringify({ username: "sam", password: "correct horse" }));
      return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    });
  render(<App />);
  fireEvent.change(await screen.findByLabelText("Usuario"), { target: { value: "sam" } });
  fireEvent.change(screen.getByLabelText("Contraseña"), { target: { value: "correct horse" } });
  fireEvent.click(screen.getByRole("button", { name: "Entrar a ModelCairn" }));
  await waitFor(() => expect(fetchMock.mock.calls.some(([url]) => url === "/api/v1/admin/session")).toBe(true));
  expect(await screen.findByRole("heading", { name: "Hola, sam" })).toBeInTheDocument();
  expect(localStorage.getItem("modelcairn-theme")).toBe("light");
  expect(localStorage.getItem("modelcairn-sidebar-collapsed")).toBe("false");
  expect(JSON.stringify(Object.entries(localStorage))).not.toContain("correct horse");
  expect(sessionStorage.length).toBe(0);
});

test("changes theme and keeps navigation in browser history", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByRole("heading", { name: "Hola, sam" })).toBeInTheDocument();
  fireEvent.click(screen.getByRole("button", { name: "Activar modo oscuro" }));
  expect(localStorage.getItem("modelcairn-theme")).toBe("dark");
  fireEvent.click(screen.getByRole("button", { name: "Contraer menú" }));
  expect(localStorage.getItem("modelcairn-sidebar-collapsed")).toBe("true");
  fireEvent.click(within(screen.getByRole("navigation", { name: "Secciones de escritorio" })).getByRole("button", { name: "Actividad" }));
  expect(window.location.hash).toBe("#/activity");
});

test("loads the secret vault from a contextual hash after refresh", async () => {
  window.location.hash = `#/secrets?from=${encodeURIComponent("#/providers/google/claves")}`;
  vi.spyOn(globalThis, "fetch").mockImplementation(input => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    if (String(input).includes("/secrets")) return response(200, { items: [], nextCursor: null });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App/>);
  expect(await screen.findByRole("heading", { name: "Claves guardadas" })).toBeInTheDocument();
  expect(screen.getByRole("link", { name: /Volver a API keys/ }).getAttribute("href")).toBe("#/providers/google/claves");
});

test("clicking the active menu returns from a detail to its list", async () => {
  window.location.hash = "#/models/google-model";
  vi.spyOn(globalThis, "fetch").mockImplementation(input => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    if (String(input).includes("/resources/")) return response(200, { items: [], nextCursor: null });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App/>);
  const navigation = await screen.findByRole("navigation", { name: "Secciones de escritorio" });
  fireEvent.click(within(navigation).getByRole("button", { name: "Modelos" }));
  expect(window.location.hash).toBe("#/models");
});

test("mobile navigation exposes the primary sections and the More menu", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(input => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    if (String(input).includes("/resources/")) return response(200, { items: [], nextCursor: null });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByRole("heading", { name: "Hola, sam" })).toBeInTheDocument();
  const mobile = screen.getByRole("navigation", { name: "Navegación móvil" });
  expect(within(mobile).getAllByRole("button")).toHaveLength(5);
  fireEvent.click(within(mobile).getByRole("button", { name: "Más" }));
  const more = screen.getByRole("menu", { name: "Más secciones" });
  expect(within(more).getByRole("menuitem", { name: "Aplicaciones" })).toBeInTheDocument();
  fireEvent.click(within(more).getByRole("menuitem", { name: "Modelos" }));
  expect(window.location.hash).toBe("#/models");
  expect(screen.queryByRole("menu", { name: "Más secciones" })).not.toBeInTheDocument();
});

test("keeps the current routes intact while the visual editor is coming soon", async () => {
  window.location.hash = "#/routes";
  vi.spyOn(globalThis, "fetch").mockImplementation(input => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: {} });
    if (String(input).includes("/resources/")) return response(200, { items: [], nextCursor: null });
    return response(200, { resourceCounts: { Route: 1 }, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByRole("heading", { name: "Rutas" })).toBeInTheDocument();
  expect(screen.getByText("Próximamente")).toBeInTheDocument();
  expect(screen.queryByRole("button", { name: "Crear con asistente" })).not.toBeInTheDocument();
  fireEvent.click(screen.getByText("Necesito editar una ruta existente"));
  fireEvent.click(screen.getByRole("link", { name: "Abrir configuración técnica" }));
  expect(await screen.findByText("Modo avanzado")).toBeInTheDocument();
});

test("shows a uniform login error", async () => {
  vi.spyOn(globalThis, "fetch")
    .mockImplementationOnce(() => response(401, { error: { code: "authentication_required" } }))
    .mockImplementationOnce(() => response(401, { error: { code: "invalid_credentials", requestId: "safe-id" } }));
  render(<App />);
  fireEvent.change(await screen.findByLabelText("Usuario"), { target: { value: "sam" } });
  fireEvent.change(screen.getByLabelText("Contraseña"), { target: { value: "wrong" } });
  fireEvent.click(screen.getByRole("button", { name: "Entrar a ModelCairn" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("El usuario o la contraseña no son correctos.");
});

test("renders the content-free operational overview", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: { persistence: { ready: true, reason: "" } } });
    return response(200, { resourceCounts: { Route: 2, Destination: 3 }, requests24h: { total: 11, success: 10, error: 1 }, activeCooldowns: 1, recentRequests: [{ id: "req-1", requestedAlias: "assistant", outcome: "success", startedAt: "2026-09-13T11:00:00Z", durationMs: 120, attempts: 1 }], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByText("11")).toBeInTheDocument();
  expect(screen.getByText("Opciones en pausa").closest("article")).toHaveTextContent("1");
  expect(screen.getByText("Opciones de ruta").closest("article")).toHaveTextContent("3");
  expect(screen.getByLabelText("Estado de la instalación").querySelectorAll(".metric")[1]).toHaveTextContent("Rutas2");
  expect(screen.getByText("Gateway").closest("article")).toHaveTextContent("Listo");
  expect(screen.getByText("Correcta")).toBeInTheDocument();
});

test("explains which local dependency is not ready", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(503, { status: "not_ready", components: { persistence: { ready: false, reason: "database unavailable" } } });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByText("La instalación necesita atención")).toBeInTheDocument();
  expect(screen.getByText("persistence: database unavailable")).toBeInTheDocument();
});
