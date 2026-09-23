import "./route-flow.css";

export type FlowStep = {
  id: string;
  provider: string;
  model: string;
  credential: string;
};

export default function RouteFlow({
  alias, steps, selected, onSelect,
}: {
  alias: string;
  steps: FlowStep[];
  selected?: string;
  onSelect?: (id: string) => void;
}) {
  return <div className="route-canvas" aria-label="Recorrido de la ruta">
    <div className="route-canvas-source">
      <small>ENTRADA</small>
      <strong>{alias || "Alias pendiente"}</strong>
      <span>Solicitud de una aplicación</span>
    </div>
    <div className="route-canvas-chain">
      {steps.map((step, index) => <div className="route-canvas-item" key={step.id}>
        {index > 0 && <div className="route-canvas-fallback"><span aria-hidden="true">↓</span><span>Error apto para fallback</span></div>}
        <div className="route-canvas-step-line">
          <button type="button" className={"route-canvas-step" + (selected === step.id ? " selected" : "")} onClick={() => onSelect?.(step.id)} disabled={!onSelect} aria-pressed={selected === step.id}>
            <span className="route-canvas-order">{index + 1}</span>
            <span className="route-canvas-step-copy"><small>{index === 0 ? "PRINCIPAL" : "RESPALDO " + index}</small><strong>{step.model || "Selecciona un modelo"}</strong><span>{step.provider || "Proveedor pendiente"} · {step.credential || "Clave pendiente"}</span></span>
          </button>
          <span className="route-canvas-success">Éxito <span aria-hidden="true">→</span></span>
        </div>
      </div>)}
    </div>
    <div className="route-canvas-output"><small>SALIDA</small><strong>Respuesta</strong><span>La primera opción que responda correctamente</span></div>
  </div>;
}
