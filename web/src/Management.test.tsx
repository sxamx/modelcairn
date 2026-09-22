import { fireEvent, render, screen, waitFor } from "@testing-library/react";
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

test("updates a resource with optimistic concurrency", async () => {
  const calls: Array<{ url:string; init?:RequestInit }> = [];
  vi.spyOn(globalThis,"fetch").mockImplementation((input,init)=>{calls.push({url:String(input),init}); if(init?.method==="PUT") return json({kind:"Provider",state:"present",metadata:{name:"openrouter",resourceVersion:3},spec:{}}); return json({items:[{kind:"Provider",state:"present",metadata:{name:"openrouter",resourceVersion:2},spec:{}}]});});
  render(<Resources/>); fireEvent.click(await screen.findByRole("button",{name:"Editar"})); fireEvent.click(screen.getByRole("button",{name:"Guardar recurso"}));
  await waitFor(()=>expect(calls.some(call=>call.init?.method==="PUT")).toBe(true));
  const update=calls.find(call=>call.init?.method==="PUT"); expect(update?.url).toContain("/resources/providers/openrouter"); expect(new Headers(update?.init?.headers).get("If-Match")).toBe('"2"');
});

test("publishes a strategy only after explicit confirmation", async () => {
  vi.spyOn(globalThis,"confirm").mockReturnValue(true);
  const calls:Array<{url:string;init?:RequestInit}>=[];
  vi.spyOn(globalThis,"fetch").mockImplementation((input,init)=>{calls.push({url:String(input),init}); const url=String(input); if(url.includes("/strategies/demo/publish")) return json({kind:"Strategy",state:"present",metadata:{name:"demo",resourceVersion:2},spec:{destinations:[]}}); if(url.includes("/resources/strategies")) return json({items:[{kind:"Strategy",state:"present",metadata:{name:"demo",resourceVersion:2},spec:{destinations:[]}}]}); return json({items:[]});});
  render(<Resources/>); fireEvent.click(screen.getByRole("tab",{name:"Estrategias"})); fireEvent.click(await screen.findByRole("button",{name:"Publicar"}));
  await waitFor(()=>expect(calls.some(call=>call.url.includes("/strategies/demo/publish")&&call.init?.method==="POST")).toBe(true));
  const publish=calls.find(call=>call.url.includes("/strategies/demo/publish")); expect(new Headers(publish?.init?.headers).get("If-Match")).toBe('"2"');
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
  const value = screen.getByLabelText("API key o secreto") as HTMLInputElement;
  fireEvent.change(value, { target: { value: "private-value" } });
  fireEvent.click(screen.getByRole("button", { name: "Guardar o reemplazar" }));
  expect(await screen.findByText(/Secreto guardado/)).toBeInTheDocument();
  expect(value.value).toBe("");
  const write = calls.find((call) => call.init?.method === "PUT");
  expect(write?.init?.body).toBe(JSON.stringify({ value: "private-value" }));
  expect(document.body.textContent).not.toContain("private-value");
  expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0);
});

test("secret vault returns to the exact provider API keys tab", async () => {
  window.location.hash = `#/secrets?from=${encodeURIComponent("#/providers/google/claves")}`;
  vi.spyOn(globalThis, "fetch").mockImplementation(() => json({ items: [], nextCursor: null }));
  render(<Secrets/>);
  expect((await screen.findByRole("link", { name: /Volver a API keys/ })).getAttribute("href")).toBe("#/providers/google/claves");
  window.location.hash = "";
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
