import { useState } from "react";
import { api, APIError, Configuration, ConfigurationPlan, Resource } from "./api/client";
import RouteFlow, { FlowModel, FlowStep } from "./RouteFlow";

type DraftStep = { id: string; model: string; credential: string };
type Pending = { configuration: Configuration; plan: ConfigurationPlan };
type Props = {
  models: Resource[];
  credentials: Resource[];
  connections: Resource[];
  accounts: Resource[];
  providers: Resource[];
  onCancel: () => void;
  onCreated: (routeName: string) => void;
};

const fields = (value: unknown): Record<string, unknown> =>
  value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
const name = (item: Resource): string => String(fields(item.metadata).name ?? "");
const label = (item: Resource): string => String(fields(item.metadata).displayName || name(item));
const spec = (item: Resource): Record<string, unknown> => "spec" in item ? fields(item.spec) : {};
const reference = (value: unknown): string => String(fields(value).name ?? "");
const enabled = (item: Resource): boolean => spec(item).enabled !== false;

export default function RouteComposer({ models, credentials, connections, accounts, providers, onCancel, onCreated }: Props) {
  const [suffix] = useState(() => crypto.randomUUID().slice(0, 12));
  const [alias, setAlias] = useState("");
  const [steps, setSteps] = useState<DraftStep[]>([]);
  const [selected, setSelected] = useState("");
  const [modalId, setModalId] = useState<string | null>(null);
  const [aliasOpen, setAliasOpen] = useState(false);
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const modelProvider = (model: Resource): string => reference(spec(connections.find(row => name(row) === reference(spec(model).connectionRef)) ?? model).providerRef);
  const keyProvider = (key: Resource): string => reference(spec(accounts.find(row => name(row) === reference(spec(key).providerAccountRef)) ?? key).providerRef);
  const availableModels = models.filter(enabled);
  const palette: FlowModel[] = availableModels.map(model => {
    const provider = providers.find(row => name(row) === modelProvider(model));
    return { id: name(model), label: label(model), provider: provider ? label(provider) : "Proveedor" };
  });
  const currentIndex = steps.findIndex(step => step.id === modalId);
  const current = steps[currentIndex];
  const currentModel = current && models.find(row => name(row) === current.model);
  const compatibleKeys = credentials.filter(key => enabled(key) && currentModel && keyProvider(key) === modelProvider(currentModel));
  const flow: FlowStep[] = steps.map(step => {
    const model = models.find(row => name(row) === step.model);
    const key = credentials.find(row => name(row) === step.credential);
    const provider = model && providers.find(row => name(row) === modelProvider(model));
    return { id: step.id, provider: provider ? label(provider) : "", model: model ? label(model) : "", credential: key ? label(key) : "" };
  });
  function changeStep(id: string, patch: Partial<DraftStep>) {
    setPending(null);
    setSteps(rows => rows.map(row => row.id === id ? { ...row, ...patch } : row));
  }
  function addModel(modelName: string, index: number) {
    if (steps.length >= 32) return;
    const model = models.find(row => name(row) === modelName);
    if (!model) return;
    const keys = credentials.filter(key => enabled(key) && keyProvider(key) === modelProvider(model));
    const next = { id: crypto.randomUUID(), model: modelName, credential: keys.length === 1 ? name(keys[0]) : "" };
    setSteps(rows => { const copy = [...rows]; copy.splice(index, 0, next); return copy; });
    setSelected(next.id);
    setModalId(next.id);
    setPending(null);
  }
  function moveStep(id: string, index: number) {
    setSteps(rows => {
      const from = rows.findIndex(row => row.id === id);
      if (from < 0 || index === from || index === from + 1) return rows;
      const copy = [...rows];
      const [item] = copy.splice(from, 1);
      copy.splice(index > from ? index - 1 : index, 0, item);
      return copy;
    });
    setPending(null);
  }
  function moveCurrent(direction: number) {
    if (!current || currentIndex + direction < 0 || currentIndex + direction >= steps.length) return;
    moveStep(current.id, currentIndex + (direction > 0 ? 2 : -1));
  }
  function removeCurrent() {
    if (!current) return;
    setSteps(rows => rows.filter(row => row.id !== current.id));
    setModalId(null);
    setSelected("");
    setPending(null);
  }
  function configuration(): Configuration {
    const routeName = "route-" + suffix;
    const strategyName = "strategy-" + suffix;
    const ref = (value: string) => ({ name: value });
    const resources: object[] = steps.map((step, index) => ({
      kind: "Destination", state: "present", metadata: { name: "option-" + suffix + "-" + (index + 1) },
      spec: { modelRef: ref(step.model), credentialRef: ref(step.credential), enabled: true, weight: 100 },
    }));
    resources.push({
      kind: "Strategy", state: "present", metadata: { name: strategyName },
      spec: { destinations: steps.map((_, index) => ref("option-" + suffix + "-" + (index + 1))), maxAttempts: steps.length, attemptTimeoutMs: 30000, totalTimeoutMs: 120000 },
    });
    resources.push({
      kind: "Route", state: "present", metadata: { name: routeName, displayName: alias.trim() },
      spec: { modelAlias: alias.trim(), strategyRef: ref(strategyName), enabled: true },
    });
    return { apiVersion: "modelcairn.io/v1alpha1", kind: "Configuration", resources } as Configuration;
  }
  async function review() {
    setError("");
    if (!/^[A-Za-z0-9._:/-]{1,128}$/.test(alias.trim())) { setError("Abre el nodo Entrada y define un alias válido antes de continuar."); setAliasOpen(true); return; }
    if (!steps.length) { setError("Añade un modelo al recorrido para crear el destino principal."); return; }
    if (steps.some(row => !row.model || !row.credential)) { setError("Abre cada destino y selecciona una clave API compatible."); return; }
    if (new Set(steps.map(row => row.model + ":" + row.credential)).size !== steps.length) { setError("No repitas la misma combinación de modelo y clave en esta ruta."); return; }
    const desired = configuration();
    setBusy(true);
    try {
      const plan = await api.planConfiguration(desired);
      setPending({ configuration: desired, plan });
    } catch (reason) {
      setError(reason instanceof APIError && reason.code === "already_exists" ? "Ese alias ya está en uso." : "No pudimos validar la ruta. Comprueba los modelos y las claves seleccionadas.");
    } finally { setBusy(false); }
  }
  async function create() {
    if (!pending) return;
    setBusy(true); setError("");
    try {
      await api.applyConfiguration(pending.configuration, pending.plan.planToken);
      onCreated("route-" + suffix);
    } catch (reason) {
      setPending(null);
      setError(reason instanceof APIError && ["plan_expired", "version_conflict", "plan_already_used"].includes(reason.code)
        ? "La configuración cambió o el plan venció. Revisa la ruta nuevamente."
        : "No pudimos crear la ruta. Revisa la configuración y vuelve a intentarlo.");
    } finally { setBusy(false); }
  }
  return <div className="route-composer">
    <button type="button" className="route-back-button" onClick={onCancel}>← Volver a rutas</button>
    <div className="route-page-head"><div><p className="eyebrow">NUEVA RUTA</p><h2 id="catalog-title">Diseña el recorrido</h2><p>Añade modelos al lienzo. Abre cada tarjeta para configurar su clave y la prioridad del fallback.</p></div><span className="catalog-badge">Sin publicar</span></div>
    {!availableModels.length && <div className="notice">Añade primero un modelo y una API key en <a href="#/providers">Proveedores</a>.</div>}
    <RouteFlow alias={alias} layoutId={"route-" + suffix} steps={flow} selected={selected} models={palette} onAddModel={addModel} onMoveStep={moveStep} sourceActionLabel="Editar alias ↗"
      onSourceClick={() => setAliasOpen(true)} onSelect={id => { setSelected(id); setModalId(id); }}/>
    {error && <div className="form-error" role="alert">{error}</div>}
    {pending && <section className="route-review" aria-label="Revisión de la ruta"><h3>Lista para crear</h3><p>{alias} tendrá {steps.length} {steps.length === 1 ? "destino" : "destinos"}; se activará al confirmar. Se crearán {pending.plan.changes.length} recursos en una sola operación.</p><div className="route-review-actions"><button type="button" className="secondary" disabled={busy} onClick={() => setPending(null)}>Seguir editando</button><button type="button" disabled={busy} onClick={create}>{busy ? "Creando…" : "Crear y activar ruta"}</button></div></section>}
    {!pending && <div className="route-review-actions"><button type="button" className="secondary" onClick={onCancel}>Cancelar</button><button type="button" disabled={busy || !availableModels.length} onClick={review}>{busy ? "Validando…" : "Revisar ruta"}</button></div>}
    {aliasOpen && <div className="route-node-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setAliasOpen(false); }}><div className="route-node-dialog" role="dialog" aria-modal="true" aria-labelledby="route-alias-title"><div className="route-node-dialog-head"><div><small>ENTRADA</small><h2 id="route-alias-title">Alias de la ruta</h2></div><button type="button" aria-label="Cerrar" onClick={() => setAliasOpen(false)}>×</button></div><p>Es el nombre de modelo que usará tu aplicación al llamar al gateway.</p><label>Alias<input autoFocus value={alias} onChange={event => { setAlias(event.target.value); setPending(null); }} placeholder="assistant" maxLength={128}/></label><div className="route-node-dialog-actions"><button type="button" onClick={() => setAliasOpen(false)}>Listo</button></div></div></div>}
    {current && <div className="route-node-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setModalId(null); }}><div className="route-node-dialog" role="dialog" aria-modal="true" aria-labelledby="route-node-title"><div className="route-node-dialog-head"><div><small>{currentIndex === 0 ? "DESTINO PRINCIPAL" : "RESPALDO " + currentIndex}</small><h2 id="route-node-title">Configurar destino</h2></div><button type="button" aria-label="Cerrar" onClick={() => setModalId(null)}>×</button></div><p>El modelo y la clave deben pertenecer al mismo proveedor. Usa Subir/Bajar para cambiar la prioridad; arrastrar la tarjeta solo organiza el dibujo.</p><label>Modelo<select value={current.model} onChange={event => changeStep(current.id, { model: event.target.value, credential: "" })}><option value="">Selecciona un modelo</option>{availableModels.map(item => <option key={name(item)} value={name(item)}>{label(item)}</option>)}</select></label><label>Clave API<select value={current.credential} disabled={!current.model} onChange={event => changeStep(current.id, { credential: event.target.value })}><option value="">Selecciona una clave</option>{compatibleKeys.map(item => <option key={name(item)} value={name(item)}>{label(item)}</option>)}</select></label>{current.model && !compatibleKeys.length && <p>Este proveedor todavía no tiene una clave compatible vinculada.</p>}<div className="route-node-dialog-actions"><button type="button" disabled={currentIndex === 0} onClick={() => moveCurrent(-1)}>↑ Subir</button><button type="button" disabled={currentIndex === steps.length - 1} onClick={() => moveCurrent(1)}>↓ Bajar</button><button type="button" className="danger" onClick={removeCurrent}>Quitar</button><button type="button" onClick={() => setModalId(null)}>Listo</button></div></div></div>}
  </div>;
}
