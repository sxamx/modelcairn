import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import App from "./App";

afterEach(() => vi.restoreAllMocks());

function response(status: number, body?: unknown) {
  return Promise.resolve(new Response(body === undefined ? null : JSON.stringify(body), {
    status,
    headers: { "Content-Type": "application/json" },
  }));
}

test("recovers an existing administrative session", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(() => response(200, {
    admin: { id: "d79b24b4-4da4-43bf-829b-940e0f025d33", username: "sam" },
    csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z",
  }));
  render(<App />);
  expect(await screen.findByRole("heading", { name: "Hola, sam" })).toBeInTheDocument();
  expect(fetch).toHaveBeenCalledWith("/api/v1/admin/session/me", expect.objectContaining({ credentials: "same-origin" }));
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
  expect(localStorage.length).toBe(0);
  expect(sessionStorage.length).toBe(0);
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
    if (String(input).endsWith("/readyz")) return response(200, { status: "ready", components: { persistence: { Ready: true, Reason: "" } } });
    return response(200, { resourceCounts: { Route: 2, Destination: 3 }, requests24h: { total: 11, success: 10, error: 1 }, activeCooldowns: 1, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByText("11")).toBeInTheDocument();
  expect(screen.getByText("Cooldowns activos").closest("article")).toHaveTextContent("1");
  expect(screen.getByText("Rutas").closest("article")).toHaveTextContent("2");
  expect(screen.getByText("Gateway").closest("article")).toHaveTextContent("Listo");
});

test("explains which local dependency is not ready", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input) => {
    if (String(input).endsWith("/session/me")) return response(200, { admin: { id: "id", username: "sam" }, csrfToken: "csrf", expiresAt: "2026-09-14T00:00:00Z" });
    if (String(input).endsWith("/readyz")) return response(503, { status: "not_ready", components: { persistence: { Ready: false, Reason: "database unavailable" } } });
    return response(200, { resourceCounts: {}, requests24h: { total: 0, success: 0, error: 0 }, activeCooldowns: 0, recentRequests: [], generatedAt: "2026-09-13T12:00:00Z" });
  });
  render(<App />);
  expect(await screen.findByText("La instalación necesita atención")).toBeInTheDocument();
  expect(screen.getByText("persistence: database unavailable")).toBeInTheDocument();
});
