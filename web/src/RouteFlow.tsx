import { PointerEvent as ReactPointerEvent, useEffect, useRef, useState } from "react";
import "./route-flow.css";

export type FlowStep = {
  id: string;
  provider: string;
  model: string;
  credential: string;
};
export type FlowModel = { id: string; label: string; provider: string };
type Point = { x: number; y: number };
type Drag = { id: string; startX: number; startY: number; origin: Point; moved: boolean };
const NODE_W = 230;
const NODE_H = 142;
const START: Point = { x: 90, y: 340 };
const positionFor = (index: number): Point => ({ x: 420 + index * 275, y: index % 2 ? 440 : 250 });
const clamp = (value: number, max: number) => Math.max(12, Math.min(max, value));

function pathBetween(from: Point, to: Point): string {
  const x1 = from.x + NODE_W;
  const y1 = from.y + NODE_H / 2;
  const x2 = to.x;
  const y2 = to.y + NODE_H / 2;
  const bend = Math.max(55, Math.abs(x2 - x1) * .48);
  return "M " + x1 + " " + y1 + " C " + (x1 + bend) + " " + y1 + ", " + (x2 - bend) + " " + y2 + ", " + x2 + " " + y2;
}
function readLayout(key: string): Record<string, Point> {
  try {
    const value = JSON.parse(localStorage.getItem(key) || "{}");
    if (!value || typeof value !== "object") return {};
    const result: Record<string, Point> = {};
    for (const [id, point] of Object.entries(value)) {
      if (point && typeof point === "object" && Number.isFinite((point as Point).x) && Number.isFinite((point as Point).y)) {
        result[id] = point as Point;
      }
    }
    return result;
  } catch { return {}; }
}

export default function RouteFlow({
  alias, layoutId, steps, selected, onSelect, onSourceClick, sourceActionLabel, models, onAddModel, onMoveStep,
}: {
  alias: string;
  layoutId?: string;
  steps: FlowStep[];
  selected?: string;
  onSelect?: (id: string) => void;
  onSourceClick?: () => void;
  sourceActionLabel?: string;
  models?: FlowModel[];
  onAddModel?: (model: string, index: number) => void;
  onMoveStep?: (id: string, index: number) => void;
}) {
  const key = "modelcairn:route-layout:" + (layoutId || alias || "new-route");
  const [layout, setLayout] = useState<Record<string, Point>>(() => readLayout(key));
  const [zoom, setZoom] = useState(1);
  const [expanded, setExpanded] = useState(false);
  const [paletteOpen, setPaletteOpen] = useState(true);
  const viewport = useRef<HTMLDivElement | null>(null);
  const drag = useRef<Drag | null>(null);
  const suppressClick = useRef(false);
  const width = Math.max(1260, (steps.length + 2) * 285 + 260);
  const height = 780;
  const output = { x: width - 320, y: 340 };
  const points = steps.map((step, index) => layout[step.id] || positionFor(index));
  const activeIds = steps.map(step => step.id).join("|");
  function fit() {
    const available = viewport.current?.clientWidth;
    if (!available) return;
    setZoom(Math.max(.65, Math.min(1, Math.floor((available - 20) / width * 100) / 100)));
  }
  useEffect(() => { fit(); }, [width]);
  useEffect(() => {
    setLayout(readLayout(key));
  }, [key]);
  useEffect(() => {
    const valid = new Set(steps.map(step => step.id));
    setLayout(current => {
      if (!Object.keys(current).some(id => !valid.has(id))) return current;
      const next = Object.fromEntries(Object.entries(current).filter(([id]) => valid.has(id)));
      try { localStorage.setItem(key, JSON.stringify(next)); } catch { /* Storage is optional. */ }
      return next;
    });
  }, [activeIds, key]);

  function beginDrag(event: ReactPointerEvent<HTMLDivElement>, id: string, index: number) {
    if (event.pointerType === "mouse" && event.button !== 0) return;
    if ((event.target as HTMLElement).closest("button")) return;
    drag.current = { id, startX: event.clientX, startY: event.clientY, origin: layout[id] || positionFor(index), moved: false };
    event.currentTarget.setPointerCapture?.(event.pointerId);
  }
  function moveDrag(event: ReactPointerEvent<HTMLDivElement>) {
    const current = drag.current;
    if (!current) return;
    const dx = (event.clientX - current.startX) / zoom;
    const dy = (event.clientY - current.startY) / zoom;
    if (!current.moved && Math.hypot(dx, dy) < 5) return;
    current.moved = true;
    setLayout(previous => ({ ...previous, [current.id]: {
      x: clamp(current.origin.x + dx, width - NODE_W - 12),
      y: clamp(current.origin.y + dy, height - NODE_H - 12),
    } }));
  }
  function endDrag() {
    if (!drag.current) return;
    if (drag.current.moved) {
      suppressClick.current = true;
      window.setTimeout(() => { suppressClick.current = false; }, 0);
      setLayout(current => {
        try { localStorage.setItem(key, JSON.stringify(current)); } catch { /* Storage is optional. */ }
        return current;
      });
    }
    drag.current = null;
  }
  const edges = steps.length
    ? [pathBetween(START, points[0]), ...points.slice(0, -1).map((point, index) => pathBetween(point, points[index + 1])), pathBetween(points[points.length - 1], output)]
    : [];

  return <section className={"route-canvas-shell" + (expanded ? " expanded" : "")} aria-label="Lienzo de la ruta">
    <div className="route-canvas-topline">
      <div><strong>Recorrido de la solicitud</strong><span>Las flechas muestran el orden de respaldo ante fallos recuperables. Mover una tarjeta solo cambia su posición visual.</span></div>
      <div className="route-canvas-top-actions">
        {onAddModel && <button type="button" className="route-canvas-add" onClick={() => setPaletteOpen(value => !value)} aria-expanded={paletteOpen}>＋ Añadir modelo</button>}
        <button type="button" className="route-canvas-add" onClick={() => { setExpanded(value => !value); window.setTimeout(fit, 0); }}>{expanded ? "Salir de pantalla completa" : "Pantalla completa"}</button>
      </div>
    </div>
    <div className="route-canvas-viewport" ref={viewport}>
      <div className="route-canvas-scroll">
        <div className="route-canvas-size" style={{ width: width * zoom, height: height * zoom }}>
          <div className="route-canvas-content" style={{ width, height, transform: "scale(" + zoom + ")" }}>
            <svg className="route-canvas-edges" width={width} height={height} aria-hidden="true">
              <defs><marker id="route-arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse"><path d="M 0 0 L 10 5 L 0 10 z" /></marker></defs>
              {edges.map((path, index) => <path key={index} d={path} className={index === 0 ? "route-canvas-edge primary" : "route-canvas-edge"} markerEnd="url(#route-arrow)" />)}
            </svg>
            <button type="button" className="route-canvas-node route-canvas-terminal source" style={{ left: START.x, top: START.y }} onClick={onSourceClick} disabled={!onSourceClick}>
              <small>ENTRADA</small><strong>{alias || "Define un alias"}</strong><span>Solicitud de la aplicación</span>{onSourceClick && <em>{sourceActionLabel || "Editar alias"}</em>}
            </button>
            {steps.map((step, index) => <div key={step.id} className={"route-canvas-node route-canvas-model" + (selected === step.id ? " selected" : "")} style={{ left: points[index].x, top: points[index].y }}
              onPointerDown={event => beginDrag(event, step.id, index)} onPointerMove={moveDrag} onPointerUp={endDrag} onPointerCancel={endDrag}>
              <div className="route-canvas-node-head"><small>{index === 0 ? "DESTINO PRINCIPAL" : "RESPALDO " + index}</small><span aria-hidden="true">⠿</span></div>
              <button type="button" onClick={() => { if (!suppressClick.current) onSelect?.(step.id); }} disabled={!onSelect} aria-label={"Configurar " + (step.model || "destino")}>
                <strong>{step.model || "Configurar modelo"}</strong><span>{step.provider || "Proveedor pendiente"}</span>
                <span className="route-canvas-key">{step.credential || "Selecciona una clave API"}</span>
              </button>
            </div>)}
            <div className="route-canvas-node route-canvas-terminal output" style={{ left: output.x, top: output.y }}>
              <small>SALIDA</small><strong>Respuesta</strong><span>Primer destino que responda</span>
            </div>
            {!steps.length && <div className="route-canvas-empty">Añade un modelo para crear el destino principal.</div>}
          </div>
        </div>
      </div>
      {paletteOpen && models && <div className="route-canvas-palette"><div className="route-canvas-palette-head"><strong>Modelos disponibles</strong><button type="button" onClick={() => setPaletteOpen(false)} aria-label="Cerrar modelos">×</button></div><p>Añade un destino; después configúralo desde su tarjeta.</p><div>{models.map(model => <button key={model.id} type="button" aria-label={"Añadir modelo " + model.label} onClick={() => onAddModel?.(model.id, steps.length)}><strong>{model.label}</strong><small>{model.provider}</small><span aria-hidden="true">＋</span></button>)}</div></div>}
      <div className="route-canvas-controls" aria-label="Zoom del lienzo">
        <button type="button" onClick={() => setZoom(value => Math.max(.65, +(value - .1).toFixed(2)))} aria-label="Alejar">−</button>
        <span>{Math.round(zoom * 100)} %</span>
        <button type="button" onClick={() => setZoom(value => Math.min(1.4, +(value + .1).toFixed(2)))} aria-label="Acercar">＋</button>
        <button type="button" onClick={fit} aria-label="Ajustar al ancho">⊞</button>
        <button type="button" onClick={() => setZoom(1)} aria-label="Restablecer zoom">⤾</button>
      </div>
    </div>
    {onMoveStep && <p className="route-flow-help">Arrastra las tarjetas para ordenar el lienzo. Para cambiar la prioridad real, abre un destino y usa «Subir» o «Bajar».</p>}
  </section>;
}
