import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import RouteFlow from "./RouteFlow";

const steps = [
  { id: "one", provider: "Google", model: "Primero", credential: "clave-a" },
  { id: "two", provider: "OpenRouter", model: "Segundo", credential: "clave-b" },
];
afterEach(() => {
  localStorage.clear();
});

test("adds a model and lets a node move visually without silently changing fallback priority", () => {
  const add = vi.fn();
  const reorder = vi.fn();
  const { container } = render(<RouteFlow alias="assistant" steps={steps}
    models={[{ id: "third", label: "Tercero", provider: "Proveedor" }]}
    onAddModel={add} onMoveStep={reorder} onSelect={() => undefined}/>);
  const model = screen.getByRole("button", { name: "Añadir modelo Tercero" });
  fireEvent.click(model);
  expect(add).toHaveBeenCalledWith("third", 2);

  const first = container.querySelector(".route-canvas-model") as HTMLElement;
  fireEvent.pointerDown(first, { pointerId: 2, pointerType: "mouse", button: 0, clientX: 100, clientY: 100 });
  fireEvent.pointerMove(first, { pointerId: 2, clientX: 160, clientY: 130 });
  fireEvent.pointerUp(first, { pointerId: 2, clientX: 160, clientY: 130 });
  expect(first.style.left).toBe("480px");
  expect(first.style.top).toBe("280px");
  expect(reorder).not.toHaveBeenCalled();
  expect(JSON.parse(localStorage.getItem("modelcairn:route-layout:assistant") || "{}").one).toEqual({ x: 480, y: 280 });
});
