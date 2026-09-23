import { fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, test, vi } from "vitest";
import RouteFlow from "./RouteFlow";

const steps = [
  { id: "one", provider: "Google", model: "Primero", credential: "clave-a" },
  { id: "two", provider: "OpenRouter", model: "Segundo", credential: "clave-b" },
];
const originalElementFromPoint = document.elementFromPoint;
afterEach(() => {
  Object.defineProperty(document, "elementFromPoint", { configurable: true, value: originalElementFromPoint });
});

test("adds a model by dropping it on the canvas and reorders a node by dragging", () => {
  const add = vi.fn();
  const reorder = vi.fn();
  const { container } = render(<RouteFlow alias="assistant" steps={steps}
    models={[{ id: "third", label: "Tercero", provider: "Proveedor" }]}
    onAddModel={add} onMoveStep={reorder} onSelect={() => undefined}/>);
  const board = container.querySelector(".route-flow-board") as HTMLElement;
  Object.defineProperty(document, "elementFromPoint", { configurable: true, value: vi.fn(() => board) });
  const model = screen.getByRole("button", { name: "Añadir modelo Tercero" });
  fireEvent.pointerDown(model, { pointerId: 1, pointerType: "mouse", button: 0, clientX: 10, clientY: 10 });
  fireEvent.pointerMove(model, { pointerId: 1, clientX: 80, clientY: 80 });
  fireEvent.pointerUp(model, { pointerId: 1, clientX: 80, clientY: 80 });
  expect(add).toHaveBeenCalledWith("third", 2);

  const second = container.querySelector('[data-route-node-index="1"]') as HTMLElement;
  vi.spyOn(second, "getBoundingClientRect").mockReturnValue({ top: 0, height: 100 } as DOMRect);
  Object.defineProperty(document, "elementFromPoint", { configurable: true, value: vi.fn(() => second) });
  const handle = screen.getByRole("button", { name: "Arrastrar Primero para cambiar prioridad" });
  fireEvent.pointerDown(handle, { pointerId: 2, pointerType: "mouse", button: 0, clientX: 10, clientY: 10 });
  fireEvent.pointerMove(handle, { pointerId: 2, clientX: 80, clientY: 80 });
  fireEvent.pointerUp(handle, { pointerId: 2, clientX: 80, clientY: 80 });
  expect(reorder).toHaveBeenCalledWith("one", 2);
});
