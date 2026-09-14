import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import Settings from "./Settings";

const spec={publicOrigin:"http://127.0.0.1:8080",listen:"127.0.0.1:8080",transport:"loopback-http",trustedProxyCidrs:[],tlsCertificatePath:"",tlsPrivateKeyPath:"",idleSeconds:1800,absoluteSeconds:43200,globalAttemptsPerMinute:30,globalBurst:5,clientAttemptsPerMinute:5,clientBurst:3,maxClientEntries:1024,clientIdleSeconds:900,failedLoginRetentionSeconds:86400,argonMemoryKiB:19456,argonIterations:2};
const document={apiVersion:"modelcairn.io/v1alpha1",kind:"AdminSettings",resourceVersion:1,spec};
const json=(body:unknown)=>Promise.resolve(new Response(JSON.stringify(body),{status:200,headers:{"Content-Type":"application/json"}}));

test("reviews and applies unlimited login retention through a single-use plan",async()=>{
  const calls:Array<{url:string;init?:RequestInit}>=[];vi.spyOn(globalThis,"fetch").mockImplementation((input,init)=>{const url=String(input);calls.push({url,init});if(url.endsWith("/settings/plan"))return json({desired:{...document,spec:{...spec,failedLoginRetentionSeconds:0}},changedFields:["failedLoginRetentionSeconds"],planToken:"settings-plan-token",expiresAt:"2026-09-13T20:00:00Z"});if(url.endsWith("/settings/apply"))return json({applied:true,appliedAt:"2026-09-13T19:00:00Z",settings:{desired:{...document,resourceVersion:2,spec:{...spec,failedLoginRetentionSeconds:0}},effective:document,restartRequired:true}});return json({desired:document,effective:document,restartRequired:false});});
  render(<Settings/>);const retention=await screen.findByLabelText(/Historial de login fallido/) as HTMLInputElement;fireEvent.change(retention,{target:{value:"0"}});fireEvent.click(screen.getByRole("button",{name:"Revisar cambios"}));expect(await screen.findByText("failedLoginRetentionSeconds")).toBeInTheDocument();
  const planned=JSON.parse(String(calls.find(call=>call.url.endsWith("/settings/plan"))?.init?.body));expect(planned.spec.failedLoginRetentionSeconds).toBe(0);
  fireEvent.click(screen.getByRole("button",{name:"Aplicar configuración"}));expect(await screen.findByText("Hay cambios guardados pendientes de reinicio.")).toBeInTheDocument();
  const apply=calls.find(call=>call.url.endsWith("/settings/apply"));expect(new Headers(apply?.init?.headers).get("X-ModelCairn-Plan-Token")).toBe("settings-plan-token");await waitFor(()=>expect(screen.queryByText("Aplicando…")).not.toBeInTheDocument());
});
