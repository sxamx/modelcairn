import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import Onboarding from "./Onboarding";

function json(body: unknown, status = 200) { return Promise.resolve(new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } })); }

test("creates the complete first route and reveals the agent token once", async () => {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  vi.spyOn(globalThis, "fetch").mockImplementation((input, init) => {
    const url = String(input); calls.push({ url, init });
    if (url.includes("/secrets/")) return json({ name: "openrouter-secret", resourceVersion: 1, updatedAt: "2026-09-13T00:00:00Z" }, 201);
    if (url.endsWith("/config/plan")) return json({ valid: true, changes: [], planToken: "plan-token-value-long-enough", expiresAt: "2026-09-13T01:00:00Z" });
    if (url.endsWith("/config/apply")) return json({ applied: true, changes: [], appliedAt: "2026-09-13T00:00:00Z" });
    return json({ token: "mc_at_v1_once-only", tokenStatus: { state: "active" } }, 201);
  });
  render(<Onboarding onClose={() => undefined} onComplete={() => undefined} />);
  fireEvent.change(screen.getByLabelText("Nombre del proveedor"), { target: { value: "OpenRouter" } });
  fireEvent.change(screen.getByLabelText(/^Identificador/), { target: { value: "openrouter" } });
  fireEvent.change(screen.getByLabelText("Modelo del proveedor"), { target: { value: "model/free" } });
  fireEvent.change(screen.getByLabelText(/^API key/), { target: { value: "secret-key-value" } });
  fireEvent.click(screen.getByRole("button", { name: "Revisar y crear" }));
  expect(await screen.findByRole("heading", { name: "Confirma los cambios" })).toBeInTheDocument();
  expect(screen.getByRole("dialog")).toHaveFocus();
  expect(calls.map((call) => call.url)).toEqual(["/api/v1/admin/config/plan"]);
  fireEvent.click(screen.getByRole("button", { name: "Confirmar y crear" }));
  expect(await screen.findByText("mc_at_v1_once-only")).toBeInTheDocument();
  expect(calls.map((call) => call.url)).toEqual([
    "/api/v1/admin/config/plan", "/api/v1/admin/secrets/openrouter-secret",
    "/api/v1/admin/config/apply", "/api/v1/admin/agent-tokens/my-agent/issue",
  ]);
  const configBodies = [calls[0],calls[2]].map((call) => String(call.init?.body));
  expect(configBodies.every((body) => !body.includes("secret-key-value"))).toBe(true);
  expect(configBodies[0]).toContain("openrouter-route");
  expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0);
});
