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
  await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(2));
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
