import { useState } from "react";
import { api, APIError, Configuration, ConfigurationPlan, Resource } from "./api/client";
import RouteFlow, { FlowStep } from "./RouteFlow";

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
  const [steps, setSteps] = useState<DraftStep[]>([{ id: "first", model: "", credential: "" }]);
  const [selected, setSelected] = useState("first");
  const [pending, setPending] = useState<Pending | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const modelProvider = (model: Resource): string => reference(spec(connections.find(row => name(row) === reference(spec(model).connectionRef)) ?? model).providerRef);
  const keyProvider = (key: Resource): string => reference(spec(accounts.find(row => name(row) === reference(spec(key).providerAccountRef)) ?? key).providerRef);
  const availableModels = models.filter(enabled);
  const currentIndex = steps.findIndex(step => step.id === selected);
  const current = steps[currentIndex] ?? steps[0];
  const currentModel = models.find(row => name(row) === current.model);
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
  function move(direction: number) {
    const next = currentIndex + direction;
    if (currentIndex < 0 || next < 0 || next >= steps.length) return;
    setPending(null);
    setSteps(rows => {
      const copy = [...rows];
      [copy[currentIndex], copy[next]] = [copy[next], copy[currentIndex]];
      return copy;
    });
  }
  function addStep() {
    if (steps.length >= 32) return;
    const id = crypto.randomUUID();
    setSteps(rows => [...rows, { id, model: "", credential: "" }]);
    setSelected(id);
    setPending(null);
  }
  function removeStep() {
    if (steps.length < 2) return;
    const next = steps.filter(row => row.id !== current.id);
    setSteps(next);
    setSelected(next[Math.max(0, currentIndex - 1)].id);
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
    if (!/^[A-Za-z0-9._:/-]{1,128}$/.test(alias.trim())) { setError("El alias debe tener entre 1 y 128 caracteres: letras, números, punto, guion, barra, dos puntos o guion bajo."); return; }
    if (steps.some(row => !row.model || !row.credential)) { setError("Completa el modelo y la clave de cada opción."); return; }
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
    <div className="route-page-head"><div><p className="eyebrow">NUEVA RUTA</p><h2 id="catalog-title">Crea un recorrido</h2><p>Tu aplicación usará un alias. ModelCairn probará cada destino en el orden que elijas.</p></div><span className="catalog-badge">Sin publicar</span></div>
    <label className="route-alias-label">Alias para tus aplicaciones<input value={alias} onChange={event => { setAlias(event.target.value); setPending(null); }} placeholder="assistant" maxLength={128}/></label>
    {!availableModels.length && <div className="notice">Añade primero un modelo y una API key en <a href="#/providers">Proveedores</a>.</div>}
    <div className="route-workspace">
      <section aria-label="Recorrido propuesto"><RouteFlow alias={alias} steps={flow} selected={selected} onSelect={setSelected}/><div className="route-canvas-actions"><button type="button" className="secondary" onClick={addStep} disabled={steps.length >= 32}>+ Añadir respaldo</button><span>{steps.length} {steps.length === 1 ? "destino" : "destinos"} en secuencia</span></div></section>
      <section className="route-inspector" aria-label="Configurar destino"><h3>{currentIndex === 0 ? "Destino principal" : "Respaldo " + currentIndex}</h3><p>Elige un modelo y una clave del mismo proveedor. La salida de red ya está vinculada a esa clave.</p>
        <label>Modelo<select value={current.model} onChange={event => changeStep(current.id, { model: event.target.value, credential: "" })}><option value="">Selecciona un modelo</option>{availableModels.map(item => <option key={name(item)} value={name(item)}>{label(item)}</option>)}</select></label>
        <label>Clave API<select value={current.credential} disabled={!current.model} onChange={event => changeStep(current.id, { credential: event.target.value })}><option value="">Selecciona una clave</option>{compatibleKeys.map(item => <option key={name(item)} value={name(item)}>{label(item)}</option>)}</select></label>
        {current.model && !compatibleKeys.length && <p>Este proveedor todavía no tiene una clave compatible vinculada.</p>}
        <div className="route-inspector-actions"><button type="button" disabled={currentIndex === 0} onClick={() => move(-1)}>↑ Subir</button><button type="button" disabled={currentIndex === steps.length - 1} onClick={() => move(1)}>↓ Bajar</button><button type="button" disabled={steps.length === 1} onClick={removeStep}>Quitar</button></div>
      </section>
    </div>
    {error && <div className="form-error" role="alert">{error}</div>}
    {pending && <section className="route-review" aria-label="Revisión de la ruta"><h3>Lista para crear</h3><p>{alias} tendrá {steps.length} {steps.length === 1 ? "destino" : "destinos"}; se activará al confirmar. Se crearán {pending.plan.changes.length} recursos en una sola operación.</p><div className="route-review-actions"><button type="button" className="secondary" disabled={busy} onClick={() => setPending(null)}>Seguir editando</button><button type="button" disabled={busy} onClick={create}>{busy ? "Creando…" : "Crear y activar ruta"}</button></div></section>}
    {!pending && <div className="route-review-actions"><button type="button" className="secondary" onClick={onCancel}>Cancelar</button><button type="button" disabled={busy || !availableModels.length} onClick={review}>{busy ? "Validando…" : "Revisar ruta"}</button></div>}
  </div>;
}
