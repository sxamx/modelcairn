import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { expect, test, vi } from "vitest";
import Activity from "./Activity";

const json=(body:unknown)=>Promise.resolve(new Response(JSON.stringify(body),{status:200,headers:{"Content-Type":"application/json"}}));
test("filters requests and expands their content-free attempts",async()=>{
  const urls:string[]=[];vi.spyOn(globalThis,"fetch").mockImplementation((input)=>{const url=String(input);urls.push(url);if(url.endsWith("/attempts"))return json({items:[{sequence:1,destinationID:"destination-id",startedAt:"2026-09-13T12:00:00Z",completedAt:"2026-09-13T12:00:01Z",outcome:"error",errorClass:"rate_limited",providerStatus:429,providerRequestID:null,retryable:true,fallbackReason:"retry_after"}]});return json({items:[{id:"request-id",requestedAlias:"assistant",startedAt:"2026-09-13T12:00:00Z",completedAt:"2026-09-13T12:00:01Z",outcome:"error",httpStatus:429,inputTokens:10,outputTokens:2,ttftMillis:80,durationMillis:1000,attempts:1}],nextCursor:null});});
  render(<Activity/>);fireEvent.click(await screen.findByText("assistant"));expect(await screen.findByText("Intento 1")).toBeInTheDocument();expect(screen.getByText("Permitió fallback")).toBeInTheDocument();
  fireEvent.change(screen.getByLabelText("Resultado"),{target:{value:"error"}});fireEvent.click(screen.getByRole("button",{name:"Aplicar filtros"}));
  await waitFor(()=>expect(urls.some(url=>url.includes("outcome=error"))).toBe(true));
  await waitFor(()=>expect(screen.queryByText("Cargando actividad…")).not.toBeInTheDocument());
  expect(document.body.textContent).not.toContain("sensitive prompt fixture");
});
