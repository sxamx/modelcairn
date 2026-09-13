import { render, screen } from "@testing-library/react";
import axe from "axe-core";
import { expect, test, vi } from "vitest";
import App from "./App";

test("login shell has no automatically detectable accessibility violations",async()=>{
  document.documentElement.lang="es";document.title="ModelCairn";
  vi.spyOn(globalThis,"fetch").mockResolvedValue(new Response(JSON.stringify({error:{code:"authentication_required"}}),{status:401,headers:{"Content-Type":"application/json"}}));
  render(<App/>);await screen.findByRole("heading",{name:"Bienvenido de vuelta"});
  const result=await axe.run(document,{rules:{"color-contrast":{enabled:false}}});
  expect(result.violations.map(item=>`${item.id}: ${item.help}`)).toEqual([]);
});
