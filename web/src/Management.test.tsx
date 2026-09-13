import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import Resources from "./Resources";
import Secrets from "./Secrets";
import AgentTokens from "./AgentTokens";

const json = (body: unknown, status = 200) => Promise.resolve(new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }));
afterEach(() => vi.restoreAllMocks());

test("lists redacted resources from the selected server kind", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation(() => json({ items: [{ kind: "Provider", state: "present", metadata: { name: "openrouter", displayName: "OpenRouter", uid: "id", resourceVersion: 2 }, spec: {} }], nextCursor: null }));
  render(<Resources />);
  expect(await screen.findByRole("heading", { name: "OpenRouter" })).toBeInTheDocument();
  expect(screen.getByText(/versión 2/)).toBeInTheDocument();
  expect(fetch).toHaveBeenCalledWith("/api/v1/admin/resources/providers?limit=200", expect.anything());
});

test("stores a secret value only in the write request and clears the field", async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
    calls.push({ url: String(input), init });
    if (init?.method === "PUT") return json({ name: "provider-key", fingerprint: "sha256:safe", resourceVersion: 1, updatedAt: "2026-09-13T00:00:00Z" }, 201);
    return json({ items: [], nextCursor: null });
  });
  render(<Secrets />);
  await screen.findByText("Todavía no hay secretos.");
  fireEvent.change(screen.getByLabelText("Nombre"), { target: { value: "provider-key" } });
  const value = screen.getByLabelText("Nuevo valor") as HTMLInputElement;
  fireEvent.change(value, { target: { value: "private-value" } });
  fireEvent.click(screen.getByRole("button", { name: "Guardar o reemplazar" }));
  expect(await screen.findByText(/Secreto guardado/)).toBeInTheDocument();
  expect(value.value).toBe("");
  const write = calls.find((call) => call.init?.method === "PUT");
  expect(write?.init?.body).toBe(JSON.stringify({ value: "private-value" }));
  expect(document.body.textContent).not.toContain("private-value");
  expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0);
});

test("issues an agent token through one-time delivery without browser persistence", async () => {
  vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
    const url = String(input);
    if (url.includes("/resources/agent-tokens")) return json({ items: [{ kind: "AgentToken", state: "present", metadata: { name: "my-agent", resourceVersion: 1 }, spec: { allowedRouteRefs: [{ name: "route" }], expiresAt: null, enabled: true } }] });
    if (url.endsWith("/status")) return json({ tokenStatus: { state: "unissued" } });
    if (url.endsWith("/issue") && init?.method === "POST") return json({ token: "mc_at_v1_one-time-value", tokenStatus: { state: "active", prefix: "mc_at_v1_one-time" } }, 201);
    throw new Error(`unexpected request ${url}`);
  });
  render(<AgentTokens />);
  fireEvent.click(await screen.findByRole("button", { name: "Emitir token" }));
  expect(await screen.findByText("mc_at_v1_one-time-value")).toBeInTheDocument();
  expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0);
});
