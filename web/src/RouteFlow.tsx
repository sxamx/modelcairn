import { PointerEvent as ReactPointerEvent, useRef, useState } from "react";
import "./route-flow.css";

export type FlowStep = {
  id: string;
  provider: string;
  model: string;
  credential: string;
};
export type FlowModel = { id: string; label: string; provider: string };
type Drag = { kind: "model" | "step"; id: string; label: string; x: number; y: number; moved: boolean };

export default function RouteFlow({
  alias, steps, selected, onSelect, onSourceClick, sourceActionLabel, models, onAddModel, onMoveStep,
}: {
  alias: string;
  steps: FlowStep[];
  selected?: string;
  onSelect?: (id: string) => void;
  onSourceClick?: () => void;
  sourceActionLabel?: string;
  models?: FlowModel[];
  onAddModel?: (model: string, index: number) => void;
  onMoveStep?: (id: string, index: number) => void;
}) {
  const drag = useRef<Drag | null>(null);
  const suppressClick = useRef(false);
  const [preview, setPreview] = useState<{ label: string; x: number; y: number } | null>(null);
  const [dropIndex, setDropIndex] = useState<number | null>(null);
  const editable = Boolean(onAddModel || onMoveStep);

  function targetIndex(x: number, y: number): number | null {
    const hit = document.elementFromPoint(x, y);
    if (!hit?.closest(".route-flow-board")) return null;
    const node = hit.closest<HTMLElement>("[data-route-node-index]");
    if (!node) return steps.length;
    const index = Number(node.dataset.routeNodeIndex);
    return y > node.getBoundingClientRect().top + node.getBoundingClientRect().height / 2 ? index + 1 : index;
  }
  function start(event: ReactPointerEvent<HTMLButtonElement>, kind: Drag["kind"], id: string, label: string) {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    drag.current = { kind, id, label, x: event.clientX, y: event.clientY, moved: false };
    event.currentTarget.setPointerCapture?.(event.pointerId);
  }
  function move(event: ReactPointerEvent<HTMLButtonElement>) {
    const current = drag.current;
    if (!current) return;
    if (!current.moved && Math.hypot(event.clientX - current.x, event.clientY - current.y) < 7) return;
    current.moved = true;
    setPreview({ label: current.label, x: event.clientX, y: event.clientY });
    setDropIndex(targetIndex(event.clientX, event.clientY));
  }
  function end(event: ReactPointerEvent<HTMLButtonElement>) {
    const current = drag.current;
    if (!current) return;
    const index = current.moved ? targetIndex(event.clientX, event.clientY) : null;
    if (current.moved) {
      suppressClick.current = true;
      window.setTimeout(() => { suppressClick.current = false; }, 0);
    }
    drag.current = null;
    setPreview(null);
    setDropIndex(null);
    if (index === null) return;
    if (current.kind === "model") onAddModel?.(current.id, index);
    else onMoveStep?.(current.id, index);
  }
  function cancel() { drag.current = null; setPreview(null); setDropIndex(null); }

  return <div className="route-flow-shell" aria-label="Editor visual del recorrido">
    {models && <div className="route-flow-palette">
      <div><strong>Modelos disponibles</strong><span>Arrastra al recorrido o toca para añadir</span></div>
      <div className="route-flow-models">{models.map(model => <button key={model.id} type="button" className="route-flow-model" aria-label={"Añadir modelo " + model.label}
        onPointerDown={event => start(event, "model", model.id, model.label)} onPointerMove={move} onPointerUp={end} onPointerCancel={cancel}
        onClick={() => { if (!suppressClick.current) onAddModel?.(model.id, steps.length); }}>
        <span className="route-flow-grip" aria-hidden="true">⠿</span><span><strong>{model.label}</strong><small>{model.provider}</small></span><span aria-hidden="true">＋</span>
      </button>)}</div>
    </div>}
    <div className="route-flow-board">
      <button type="button" className="route-flow-terminal route-flow-source" onClick={onSourceClick} disabled={!onSourceClick}>
        <small>ENTRADA</small><strong>{alias || "Define un alias"}</strong><span>Solicitud de la aplicación</span>{onSourceClick && <em>{sourceActionLabel || "Ver detalles ↗"}</em>}
      </button>
      <div className="route-flow-track">
        {!steps.length && <div className="route-flow-empty">Arrastra un modelo aquí para crear el destino principal.</div>}
        {steps.map((step, index) => <div key={step.id} data-route-node-index={index} className="route-flow-item">
          {dropIndex === index && <span className="route-flow-drop-line" aria-hidden="true"/>}
          {index > 0 && <div className="route-flow-fallback"><span className="route-flow-connector" aria-hidden="true"/><span>Si el fallo permite respaldo</span></div>}
          <div className={"route-flow-node" + (selected === step.id ? " selected" : "")}>
            {onMoveStep && <button type="button" className="route-flow-drag" aria-label={"Arrastrar " + (step.model || "destino") + " para cambiar prioridad"}
              onPointerDown={event => start(event, "step", step.id, step.model || "Destino")} onPointerMove={move} onPointerUp={end} onPointerCancel={cancel}>⠿</button>}
            <button type="button" className="route-flow-open" onClick={() => onSelect?.(step.id)} disabled={!onSelect} aria-label={"Configurar " + (step.model || "destino")}>
              <span className="route-flow-index">{index + 1}</span>
              <span className="route-flow-node-text"><small>{index === 0 ? "DESTINO PRINCIPAL" : "RESPALDO " + index}</small><strong>{step.model || "Configura el modelo"}</strong><span>{step.provider || "Proveedor pendiente"} · {step.credential || "Clave pendiente"}</span></span>
              <span className="route-flow-edit" aria-hidden="true">↗</span>
            </button>
          </div>
          <span className="route-flow-success">Éxito → respuesta</span>
        </div>)}
        {dropIndex === steps.length && <span className="route-flow-drop-line" aria-hidden="true"/>}
      </div>
      <div className="route-flow-terminal route-flow-output"><small>SALIDA</small><strong>Respuesta</strong><span>Primer destino que responda</span></div>
    </div>
    {editable && <p className="route-flow-help">El orden de arriba abajo es la prioridad real. Puedes abrir cada nodo para editarlo.</p>}
    {preview && <div className="route-flow-drag-preview" style={{ left: preview.x + 14, top: preview.y + 14 }}>{preview.label}</div>}
  </div>;
}
