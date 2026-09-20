import { fireEvent, render, screen } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import Onboarding from "./Onboarding";
import fixture from "../../docs/contratos/config/onboarding-first-route-v1.json";
import { buildOnboardingConfiguration } from "./onboarding-configuration";

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
  const planned = JSON.parse(configBodies[0]);
  expect(planned.resources.find((item:{kind:string})=>item.kind==="Model").spec.capabilities).toEqual(["text"]);
  expect(localStorage.length).toBe(0); expect(sessionStorage.length).toBe(0);
});

test("shared onboarding fixture requires explicit streaming and tools", () => {
  expect(buildOnboardingConfiguration("onboarding","Example Provider","https://api.example.invalid/v1","example-chat-model","onboarding-secret","assistant","onboarding-agent",{stream:true,tools:true})).toEqual(fixture);
  const conservative = buildOnboardingConfiguration("onboarding","Example Provider","https://api.example.invalid/v1","example-chat-model","onboarding-secret","assistant","onboarding-agent");
  const model = conservative.resources.find((item)=>item.kind==="Model") as unknown as {spec:{capabilities:string[]}};
  expect(model.spec.capabilities).toEqual(["text"]);
});

test("reconciles a lost apply response before issuing the token", async () => {
  let plans=0;
  vi.spyOn(globalThis,"fetch").mockImplementation((input)=>{
    const url=String(input);
    if(url.endsWith("/config/plan")){plans++;return json({valid:true,changes:plans===1?[{operation:"create",kind:"Route",name:"route"}]:[],planToken:`plan-${plans}-token-long-enough`,expiresAt:"2026-09-13T01:00:00Z"});}
    if(url.includes("/secrets/"))return json({name:"demo-secret",resourceVersion:1,updatedAt:"2026-09-13T00:00:00Z"},201);
    if(url.endsWith("/config/apply"))return json({error:{code:"unavailable"}},503);
    return json({token:"mc_at_v1_reconciled",tokenStatus:{state:"active"}},201);
  });
  render(<Onboarding onClose={()=>undefined} onComplete={()=>undefined}/>);
  fireEvent.change(screen.getByLabelText("Nombre del proveedor"),{target:{value:"Demo"}});fireEvent.change(screen.getByLabelText(/^Identificador/),{target:{value:"demo"}});fireEvent.change(screen.getByLabelText("Modelo del proveedor"),{target:{value:"demo-model"}});fireEvent.change(screen.getByLabelText(/^API key/),{target:{value:"secret-key-value"}});fireEvent.click(screen.getByRole("button",{name:"Revisar y crear"}));await screen.findByRole("heading",{name:"Confirma los cambios"});fireEvent.click(screen.getByRole("button",{name:"Confirmar y crear"}));
  expect(await screen.findByText("mc_at_v1_reconciled")).toBeInTheDocument();expect(plans).toBe(2);
});

test("rotates a token whose one-time response was lost", async () => {
  let issues=0;
  vi.spyOn(globalThis,"fetch").mockImplementation((input)=>{const url=String(input);if(url.endsWith("/config/plan"))return json({valid:true,changes:[],planToken:"plan-token-value-long-enough",expiresAt:"2026-09-13T01:00:00Z"});if(url.includes("/secrets/"))return json({name:"demo-secret",resourceVersion:1,updatedAt:"2026-09-13T00:00:00Z"},201);if(url.endsWith("/config/apply"))return json({applied:true,changes:[],appliedAt:"2026-09-13T00:00:00Z"});if(url.endsWith("/status"))return json({tokenStatus:{state:"active"}});if(url.endsWith("/rotate"))return json({token:"mc_at_v1_replacement",tokenStatus:{state:"active"}},201);if(url.endsWith("/issue")){issues++;return json({error:{code:"unavailable"}},503);}return json({});});
  render(<Onboarding onClose={()=>undefined} onComplete={()=>undefined}/>);fireEvent.change(screen.getByLabelText("Nombre del proveedor"),{target:{value:"Demo"}});fireEvent.change(screen.getByLabelText(/^Identificador/),{target:{value:"demo"}});fireEvent.change(screen.getByLabelText("Modelo del proveedor"),{target:{value:"demo-model"}});fireEvent.change(screen.getByLabelText(/^API key/),{target:{value:"secret-key-value"}});fireEvent.click(screen.getByRole("button",{name:"Revisar y crear"}));await screen.findByRole("heading",{name:"Confirma los cambios"});fireEvent.click(screen.getByRole("button",{name:"Confirmar y crear"}));expect(await screen.findByRole("heading",{name:"Recupera el acceso"})).toBeInTheDocument();fireEvent.click(screen.getByRole("button",{name:"Rotar y mostrar token nuevo"}));expect(await screen.findByText("mc_at_v1_replacement")).toBeInTheDocument();expect(issues).toBe(1);
});
