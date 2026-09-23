import { FormEvent, useCallback, useEffect, useState } from "react";
import { api, APIError, OperationalMetrics, Resource } from "./api/client";
import Resources, { ResourceArea } from "./Resources";
import ProviderConnectionForm from "./ProviderConnectionForm";
import ProviderKeyLink from "./ProviderKeyLink";
import RouteComposer from "./RouteComposer";
import RouteFlow, { FlowStep } from "./RouteFlow";

type Area = Exclude<ResourceArea, "all">;
type Kind = "providers" | "provider-accounts" | "provider-connections" | "credentials" | "egresses" | "models" | "destinations" | "strategies" | "routes";
type CatalogData = Record<Kind, Resource[]>;
const kinds: Kind[] = ["providers", "provider-accounts", "provider-connections", "credentials", "egresses", "models", "destinations", "strategies", "routes"];
async function listAll(kind: Kind): Promise<Resource[]> {
  const rows: Resource[] = [];
  let cursor: string | undefined;
  let pages = 0;
  do {
    const page = await api.listResources(kind, cursor);
    rows.push(...page.items);
    cursor = page.nextCursor || undefined;
    pages++;
    if (pages > 50) throw new Error("resource_list_too_large");
  } while (cursor);
  return rows;
}
const blank = (): CatalogData => ({
  providers: [], "provider-accounts": [], "provider-connections": [], credentials: [],
  egresses: [], models: [], destinations: [], strategies: [], routes: [],
});
const title: Record<Area, string> = { providers: "Proveedores", models: "Modelos", routes: "Rutas" };

function object(value: unknown): Record<string, unknown> {
  return value && typeof value === "object" && !Array.isArray(value) ? value as Record<string, unknown> : {};
}
function name(resource: Resource): string { return String(object(resource.metadata).name ?? ""); }
function display(resource: Resource): string { return String(object(resource.metadata).displayName || name(resource)); }
function spec(resource: Resource): Record<string, unknown> { return "spec" in resource ? object(resource.spec) : {}; }
function ref(value: unknown): string { return String(object(value).name ?? ""); }
function text(value: unknown): string { return typeof value === "string" ? value : ""; }
function enabled(resource: Resource): boolean { return spec(resource).enabled !== false; }
function capabilities(resource: Resource): string { const value = spec(resource).capabilities; return Array.isArray(value) ? value.filter(x => typeof x === "string").join(" · ") : "Sin especificar"; }
function find(rows: Resource[], id: string): Resource | undefined { return rows.find(row => name(row) === id); }
function url(area: Area, id?: string, tab?: string): string { return `#/${area}${id ? `/${encodeURIComponent(id)}` : ""}${tab ? `/${tab}` : ""}`; }
function modelUrl(model: string, provider?: string): string {
  const target = url("models", model);
  return provider ? `${target}?from=${encodeURIComponent(url("providers", provider, "modelos"))}` : target;
}
function backFrom(hash: string, fallback: string): string {
  const from = new URLSearchParams(hash.split("?")[1] ?? "").get("from");
  return from?.startsWith("#/providers/") ? from : fallback;
}
function modelBackLabel(target: string): string { return target.startsWith("#/providers/") ? "Volver al proveedor" : "Volver a modelos"; }

export default function Catalog({ area, onOpenWizard, onOpenSecrets }: { area: Area; onOpenWizard: () => void; onOpenSecrets: () => void }) {
  const [data, setData] = useState<CatalogData>(blank);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [location, setLocation] = useState(window.location.hash);
  const [creating, setCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [createError, setCreateError] = useState("");
  const [modelCreating, setModelCreating] = useState<{ providerId?: string } | null>(null);
  const [connectionCreating, setConnectionCreating] = useState<string | null>(null);
  const [keyLinking, setKeyLinking] = useState<string | null>(null);
  const [routeCreating, setRouteCreating] = useState(false);
  const refresh = useCallback(async (showLoading = true) => {
    if (showLoading) setLoading(true);
    setError("");
    try {
      const pages = await Promise.all(kinds.map(listAll));
      setData(Object.fromEntries(kinds.map((kind, index) => [kind, pages[index]])) as CatalogData);
    } catch { setError("No pudimos cargar la configuración. Revisa la conexión e inténtalo de nuevo."); }
    finally { if (showLoading) setLoading(false); }
  }, []);
  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    const sync = () => { setLocation(window.location.hash); setCreating(false); setModelCreating(null); setConnectionCreating(null); setKeyLinking(null); setRouteCreating(false); };
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);
  const parts = location.slice(2).split("?")[0].split("/");
  const id = parts[1] ? decodeURIComponent(parts[1]) : "";
  const tab = parts[2] || "resumen";
  const providerForModel = (row: Resource) => ref(spec(find(data["provider-connections"], ref(spec(row).connectionRef)) ?? row).providerRef);
  const providerForCredential = (row: Resource) => ref(spec(find(data["provider-accounts"], ref(spec(row).providerAccountRef)) ?? row).providerRef);
  const modelsFor = (provider: string) => data.models.filter(row => providerForModel(row) === provider);
  const keysFor = (provider: string) => data.credentials.filter(row => providerForCredential(row) === provider);
  const destinationsFor = (model: string) => data.destinations.filter(row => ref(spec(row).modelRef) === model);
  const stepsFor = (route: Resource) => {
    const strategy = find(data.strategies, ref(spec(route).strategyRef));
    const destinations = spec(strategy ?? route).destinations;
    return Array.isArray(destinations) ? destinations.map(item => find(data.destinations, ref(item))).filter((row): row is Resource => Boolean(row)) : [];
  };
  async function createProvider(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); const form = event.currentTarget; const fields = new FormData(form);
    const id = String(fields.get("name") ?? "").trim();
    const visible = String(fields.get("displayName") ?? "").trim();
    setSaving(true); setCreateError("");
    try {
      await api.createResource("providers", { kind: "Provider", state: "present", metadata: { name: id, displayName: visible || id }, spec: {} } as Resource);
      await refresh(); window.location.hash = url("providers", id);
    } catch (reason) {
      setCreateError(reason instanceof APIError && reason.status === 409 ? "Ese identificador ya existe. Elige otro." : "No pudimos crear el proveedor. Revisa los datos e inténtalo otra vez.");
    } finally { setSaving(false); }
  }
  async function createModel(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const modelName = String(fields.get("name") ?? "").trim();
    const connection = String(fields.get("connection") ?? "");
    const capabilities = ["text", ...["stream", "tools"].filter(value => fields.get(value) === "on")];
    const input = String(fields.get("inputPrice") ?? "").trim();
    const output = String(fields.get("outputPrice") ?? "").trim();
    if ((input === "") !== (output === "")) { setCreateError("Completa ambos precios o deja ambos vacíos."); return; }
    const pricing = input !== "" && output !== "" ? { currency: "USD", inputPerMillion: Number(input), outputPerMillion: Number(output) } : undefined;
    setSaving(true); setCreateError("");
    try {
      await api.createResource("models", { kind: "Model", state: "present", metadata: { name: modelName, displayName: String(fields.get("displayName") ?? "").trim() || modelName }, spec: { connectionRef: { name: connection }, providerModelId: String(fields.get("providerModelId") ?? "").trim(), capabilities, enabled: true, ...(pricing ? { pricing } : {}) } } as Resource);
      await refresh();
      const provider = ref(spec(find(data["provider-connections"], connection) ?? { spec: {} } as Resource).providerRef);
      window.location.hash = modelUrl(modelName, provider || undefined);
      setModelCreating(null);
    } catch (reason) {
      setCreateError(reason instanceof APIError && reason.status === 409 ? "Ese identificador ya existe." : "No pudimos guardar el modelo. Comprueba el identificador y la conexión.");
    } finally { setSaving(false); }
  }
  const advanced = id === "advanced";
  const intro = area === "providers" ? "Tus servicios de IA, sus modelos y las claves vinculadas, agrupados en un solo lugar."
    : area === "models" ? "Catálogo de modelos configurados. Cada modelo pertenece a una conexión de proveedor."
    : "Nombres para tus aplicaciones y orden de respaldo entre modelos y claves.";
  return <section className="catalog" aria-labelledby="catalog-title">
    {advanced ? <><a className="catalog-back" href={url(area)}>← Volver a {title[area].toLowerCase()}</a><div className="catalog-advanced-note"><strong>Modo avanzado</strong><p>Aquí se editan las entidades internas y sus referencias. Los cambios se validan en el servidor; vuelve a la vista principal para navegar por proveedor, modelo o ruta.</p></div><Resources key={area} area={area} onOpenWizard={onOpenWizard}/></> :
      <>
        {!(area === "routes" && (routeCreating || id)) && <div className="catalog-heading"><div><h1 id="catalog-title">{title[area]}</h1><p className="lede">{intro}</p></div><div className="catalog-heading-actions">{area === "providers" && !id && <button onClick={() => { setCreateError(""); setCreating(true); }}>Añadir proveedor</button>}{area === "models" && !id && <button onClick={() => { setCreateError(""); setModelCreating({}); }}>Añadir modelo</button>}{area === "routes" && !id && <button onClick={() => setRouteCreating(true)}>Nueva ruta</button>}</div></div>}
        {error && <div className="form-error" role="alert">{error} <button className="catalog-inline-button" onClick={() => void refresh()}>Reintentar</button></div>}
        {loading ? <div className="loading" role="status"><span className="spinner"/>Cargando configuración…</div> :
          area === "routes" && routeCreating ? <RouteComposer models={data.models} credentials={data.credentials} connections={data["provider-connections"]} accounts={data["provider-accounts"]} providers={data.providers} onCancel={() => setRouteCreating(false)} onCreated={routeName => { void refresh().then(() => { setRouteCreating(false); window.location.hash = url("routes", routeName); }); }}/> :
          area === "providers" ? id ? <ProviderDetail id={id} tab={tab} data={data} models={modelsFor(id)} keys={keysFor(id)} onOpenSecrets={onOpenSecrets} onAddModel={() => { setCreateError(""); setModelCreating({ providerId: id }); }} onAddConnection={() => setConnectionCreating(id)} onLinkKey={() => setKeyLinking(id)}/> :
            <><div className="catalog-grid">{data.providers.map(provider => {
              const pid = name(provider), models = modelsFor(pid), keys = keysFor(pid);
              const connections = data["provider-connections"].filter(row => ref(spec(row).providerRef) === pid);
              return <a className="catalog-card" key={pid} href={url("providers", pid)}><div className="catalog-card-top"><strong>{display(provider)}</strong><span className={connections.length && keys.length && models.length ? "catalog-badge ready" : "catalog-badge"}>{connections.length && keys.length && models.length ? "Configurado" : "Pendiente"}</span></div><p>{pid}</p><div className="catalog-card-stats"><span>{keys.length} {keys.length === 1 ? "clave" : "claves"}</span><span>{models.length} {models.length === 1 ? "modelo" : "modelos"}</span></div></a>;
            })}</div>{!data.providers.length && <Empty title="Aún no hay proveedores" message="Añade un proveedor para comenzar; puedes conectar sus modelos y claves después."/>}</> :
          area === "models" ? id ? <ModelDetail id={id} data={data} providerForModel={providerForModel} destinationsFor={destinationsFor} from={backFrom(location, url("models"))} onSaved={() => refresh(false)}/> :
            <><div className="catalog-table-wrap"><table><thead><tr><th>Modelo</th><th>Proveedor</th><th>Capacidades</th><th>Destinos configurados</th></tr></thead><tbody>{data.models.map(row => <tr key={name(row)}><td><a href={url("models", name(row))}>{display(row)}</a></td><td>{display(find(data.providers, providerForModel(row)) ?? row)}</td><td>{capabilities(row)}</td><td>{destinationsFor(name(row)).length}</td></tr>)}</tbody></table></div>{!data.models.length && <Empty title="Aún no hay modelos" message="Los modelos que configures en cada proveedor aparecerán aquí."/>}</> :
          id ? <RouteDetail id={id} data={data} stepsFor={stepsFor} onSaved={() => refresh(false)}/> :
            <><div className="route-card-list">{data.routes.map(row => {
              const steps = stepsFor(row);
              return <a className="route-list-card" key={name(row)} href={url("routes", name(row))}><div className="route-list-card-head"><strong>{text(spec(row).modelAlias) || display(row)}</strong><span className={enabled(row) ? "catalog-badge ready" : "catalog-badge"}>{enabled(row) ? "Habilitada" : "Deshabilitada"}</span></div><div className="route-list-card-flow"><span>Entrada</span>{steps.map((step, index) => <span key={name(step)}>{index ? "Respaldo · " : "Principal · "}{display(find(data.models, ref(spec(step).modelRef)) ?? step)}</span>)}<span>Respuesta</span></div></a>;
            })}</div>{!data.routes.length && <Empty title="Aún no hay rutas" message="Crea una ruta con un destino principal y añade respaldos cuando los necesites."/>}</>
        }
        {!loading && !routeCreating && <div className="catalog-footer"><a href={url(area, "advanced")}>Opciones avanzadas</a><span>Para editar relaciones técnicas o configuraciones JSON.</span></div>}
        {creating && <div className="catalog-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setCreating(false); }}><form className="catalog-dialog" onSubmit={createProvider} role="dialog" aria-modal="true" aria-labelledby="new-provider-title"><div className="catalog-dialog-head"><h2 id="new-provider-title">Añadir proveedor</h2><button type="button" aria-label="Cerrar" onClick={() => setCreating(false)}>×</button></div><p>Primero crea el proveedor. Después podrás añadir sus conexiones, modelos y claves.</p>{createError && <div className="form-error" role="alert">{createError}</div>}<label>Identificador técnico<input name="name" required pattern="[a-z][a-z0-9-]{0,62}" placeholder="mi-proveedor"/></label><label>Nombre visible<input name="displayName" maxLength={120} placeholder="Mi proveedor"/></label><div className="catalog-dialog-actions"><button type="button" className="secondary" onClick={() => setCreating(false)}>Cancelar</button><button disabled={saving}>{saving ? "Creando…" : "Crear proveedor"}</button></div></form></div>}
        {modelCreating && <div className="catalog-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setModelCreating(null); }}><form className="catalog-dialog catalog-dialog-wide" onSubmit={createModel} role="dialog" aria-modal="true" aria-labelledby="new-model-title"><div className="catalog-dialog-head"><h2 id="new-model-title">Añadir modelo</h2><button type="button" aria-label="Cerrar" onClick={() => setModelCreating(null)}>×</button></div><p>El modelo se asocia a una conexión API del proveedor. Las tarifas son opcionales y sirven solo para estimaciones.</p>{createError && <div className="form-error" role="alert">{createError}</div>}<label>Conexión API<select name="connection" required defaultValue=""> <option value="" disabled>Selecciona una conexión</option>{data["provider-connections"].filter(row => !modelCreating.providerId || ref(spec(row).providerRef) === modelCreating.providerId).map(row => <option key={name(row)} value={name(row)}>{display(find(data.providers, ref(spec(row).providerRef)) ?? row)} · {name(row)}</option>)}</select></label>{!data["provider-connections"].some(row => !modelCreating.providerId || ref(spec(row).providerRef) === modelCreating.providerId) && <p className="catalog-hint">Primero añade una conexión en la pestaña Configuración de este proveedor.</p>}<div className="catalog-form-grid"><label>Identificador interno<input name="name" required pattern="[a-z][a-z0-9-]{0,62}" placeholder="gemini-flash"/></label><label>Nombre visible<input name="displayName" maxLength={120} placeholder="Gemini Flash"/></label></div><label>ID exacto del modelo en el proveedor<input name="providerModelId" required maxLength={255} placeholder="gemini-..."/></label><fieldset><legend>Capacidades verificadas</legend><label><input type="checkbox" checked readOnly/> Texto</label><label><input type="checkbox" name="stream"/> Streaming</label><label><input type="checkbox" name="tools"/> Herramientas</label></fieldset><div className="catalog-form-grid"><label>Entrada · USD por 1 M tokens<input name="inputPrice" type="number" min="0" max="1000000" step="any" placeholder="Opcional"/></label><label>Salida · USD por 1 M tokens<input name="outputPrice" type="number" min="0" max="1000000" step="any" placeholder="Opcional"/></label></div><small>Para un modelo gratuito, escribe 0 en ambos campos. Deja ambos vacíos si el precio es desconocido.</small><div className="catalog-dialog-actions"><button type="button" className="secondary" onClick={() => setModelCreating(null)}>Cancelar</button><button disabled={saving || !data["provider-connections"].length}>{saving ? "Guardando…" : "Guardar modelo"}</button></div></form></div>}
        {connectionCreating && <ProviderConnectionForm providerId={connectionCreating} onClose={() => setConnectionCreating(null)} onSaved={() => refresh(false)}/>}
        {keyLinking && <ProviderKeyLink providerId={keyLinking} accounts={data["provider-accounts"]} egresses={data.egresses} onClose={() => setKeyLinking(null)} onOpenSecrets={onOpenSecrets} onSaved={() => refresh(false)}/>}
      </>}
  </section>;
}

function Empty({ title, message }: { title: string; message: string }) { return <div className="catalog-empty"><h2>{title}</h2><p>{message}</p></div>; }

function ProviderDetail({ id, tab, data, models, keys, onOpenSecrets, onAddModel, onAddConnection, onLinkKey }: { id: string; tab: string; data: CatalogData; models: Resource[]; keys: Resource[]; onOpenSecrets: () => void; onAddModel: () => void; onAddConnection: () => void; onLinkKey: () => void }) {
  const [metrics, setMetrics] = useState<OperationalMetrics | null>(null);
  const [metricsState, setMetricsState] = useState<"loading" | "ready" | "error">("loading");
  useEffect(() => {
    if (tab !== "resumen") return;
    let active = true;
    setMetricsState("loading");
    api.metrics().then(value => { if (!Array.isArray(value.models)) throw new Error("invalid_metrics"); if (active) { setMetrics(value); setMetricsState("ready"); } })
      .catch(() => { if (active) setMetricsState("error"); });
    return () => { active = false; };
  }, [id, tab]);
  const provider = find(data.providers, id);
  if (!provider) return <><a className="catalog-back" href={url("providers")}>← Proveedores</a><Empty title="Proveedor no encontrado" message="Puede haber sido eliminado desde otra sesión."/></>;
  const connections = data["provider-connections"].filter(row => ref(spec(row).providerRef) === id);
  const observed = models.map(model => ({ id: name(model), label: display(model), count: metrics?.models.find(row => row.name === name(model))?.requests ?? 0 }))
    .filter(row => row.count > 0).sort((left, right) => right.count - left.count);
  const largestObserved = Math.max(1, ...observed.map(row => row.count));
  const totalObserved = observed.reduce((total, row) => total + row.count, 0);
  const tabs = [["resumen","Resumen"],["modelos","Modelos"],["claves","API keys"],["configuracion","Configuración"]];
  return <><a className="catalog-back" href={url("providers")}>← Proveedores</a><div className="catalog-detail-head"><h2>{display(provider)}</h2><span className={connections.length && keys.length && models.length ? "catalog-badge ready" : "catalog-badge"}>{connections.length && keys.length && models.length ? "Configurado" : "Pendiente"}</span></div><div className="catalog-tabs" role="tablist" aria-label="Detalles del proveedor">{tabs.map(([key,label]) => <a role="tab" aria-selected={tab === key} key={key} href={url("providers", id, key)}>{label}</a>)}</div>
    {tab === "modelos" ? <><div className="catalog-section-head"><span>{models.length} {models.length === 1 ? "modelo configurado" : "modelos configurados"}</span><div><button onClick={onAddModel}>+ Añadir modelo</button><a href={url("models")}>Ver catálogo completo</a></div></div><div className="catalog-table-wrap"><table><thead><tr><th>Modelo</th><th>Identificador en el proveedor</th><th>Capacidades</th></tr></thead><tbody>{models.map(row => <tr key={name(row)}><td><a href={modelUrl(name(row), id)}>{display(row)}</a></td><td>{text(spec(row).providerModelId)}</td><td>{capabilities(row)}</td></tr>)}</tbody></table></div>{!models.length && <Empty title="Sin modelos" message="Añade aquí el primer modelo. La detección automática aún no está disponible."/>}</> :
    tab === "claves" ? <><div className="catalog-section-head"><span>{keys.length} {keys.length === 1 ? "clave vinculada" : "claves vinculadas"}</span><div><button onClick={onLinkKey}>+ Vincular API key</button><button onClick={onOpenSecrets}>Claves guardadas</button></div></div><div className="catalog-table-wrap"><table><thead><tr><th>Clave vinculada</th><th>Secreto guardado</th><th>Salida de red</th><th>Estado</th></tr></thead><tbody>{keys.map(row => <tr key={name(row)}><td>{display(row)}</td><td>{ref(spec(row).secretRef) || "—"}</td><td>{ref(spec(row).egressRef) || "—"}</td><td><span className={enabled(row) ? "catalog-badge ready" : "catalog-badge"}>{enabled(row) ? "Habilitada" : "Deshabilitada"}</span></td></tr>)}</tbody></table></div>{!keys.length && <Empty title="Sin claves vinculadas" message="Guarda una clave y usa «Vincular API key» para asociarla a este proveedor."/>}</> :
    tab === "configuracion" ? <><div className="catalog-section-head"><span>{connections.length} {connections.length === 1 ? "conexión API" : "conexiones API"}</span><button onClick={onAddConnection}>+ Añadir conexión</button></div><div className="catalog-info-grid"><article><h3>Identificador</h3><p>{name(provider)}</p></article><article><h3>Conexiones API</h3>{connections.length ? connections.map(row => <p key={name(row)}>{text(spec(row).baseUrl) || name(row)}</p>) : <p>Sin conexiones</p>}</article><article><h3>Descripción</h3><p>{text(object(provider.metadata).description) || "Sin descripción"}</p></article></div></> :
    <><div className="catalog-summary"><article><h3>Modelos</h3><strong>{models.length}</strong><p>Configurados en este proveedor</p></article><article><h3>API keys</h3><strong>{keys.length}</strong><p>Vinculadas a sus cuentas</p></article><article><h3>Conexiones</h3><strong>{connections.length}</strong><p>URLs de API configuradas</p></article><article><h3>Uptime</h3><strong>Sin sondeos</strong><p>Aún no se mide de forma independiente</p></article></div><section className="catalog-provider-usage" aria-labelledby="provider-usage-title"><div className="catalog-provider-usage-head"><div><h3 id="provider-usage-title">Solicitudes por modelo</h3><p>Historial retenido con modelo identificado; no equivale a uptime.</p></div>{metricsState === "ready" && <strong>{totalObserved.toLocaleString("es")} solicitudes</strong>}</div>{metricsState === "loading" ? <p>Cargando datos observados…</p> : metricsState === "error" ? <p>No se pudieron cargar las métricas de este proveedor.</p> : observed.length ? <div className="catalog-usage-bars">{observed.map(row => <div className="catalog-usage-row" key={row.id}><span>{row.label}</span><div className="catalog-usage-track" aria-hidden="true"><span style={{ width: `${Math.max(4, row.count / largestObserved * 100)}%` }}/></div><strong>{row.count.toLocaleString("es")}</strong></div>)}</div> : <p>Aún no hay solicitudes observadas para sus modelos.</p>}</section></>}
  </>;
}

function ModelDetail({ id, data, providerForModel, destinationsFor, from, onSaved }: { id: string; data: CatalogData; providerForModel: (row: Resource) => string; destinationsFor: (id: string) => Resource[]; from: string; onSaved: () => Promise<void> }) {
  const [usage, setUsage] = useState<OperationalMetrics["models"][number] | null>(null);
  const [usageError, setUsageError] = useState(false);
  const [priceError, setPriceError] = useState("");
  const [priceMessage, setPriceMessage] = useState("");
  const [savingPrice, setSavingPrice] = useState(false);
  useEffect(() => {
    let active = true;
    api.metrics().then(result => { if (active) setUsage(result.models.find(row => row.name === id) ?? null); }).catch(() => { if (active) setUsageError(true); });
    return () => { active = false; };
  }, [id]);
  const model = find(data.models, id);
  if (!model) return <><a className="catalog-back" href={from}>← {modelBackLabel(from)}</a><Empty title="Modelo no encontrado" message="Puede haber sido eliminado desde otra sesión."/></>;
  const currentModel = model;
  const provider = find(data.providers, providerForModel(model));
  const destinations = destinationsFor(id);
  const pricing = object(spec(model).pricing);
  async function savePricing(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const configured = fields.get("configured") === "on";
    const input = String(fields.get("inputPrice") ?? "").trim();
    const output = String(fields.get("outputPrice") ?? "").trim();
    if (configured && (input === "" || output === "")) { setPriceError("Completa ambos precios."); return; }
    const nextSpec = { ...spec(currentModel) };
    if (configured) nextSpec.pricing = { currency: "USD", inputPerMillion: Number(input), outputPerMillion: Number(output) };
    else delete nextSpec.pricing;
    setSavingPrice(true); setPriceError(""); setPriceMessage("");
    try {
      await api.updateResource("models", id, Number(object(currentModel.metadata).resourceVersion), { kind: "Model", state: "present", metadata: { name: id, displayName: object(currentModel.metadata).displayName, description: object(currentModel.metadata).description }, spec: nextSpec } as Resource);
      await onSaved(); setPriceMessage("Tarifa guardada. Las estimaciones usan el precio actual, no un historial de tarifas.");
    } catch { setPriceError("No se pudo guardar la tarifa. Recarga la página y vuelve a intentarlo."); }
    finally { setSavingPrice(false); }
  }
  return <><a className="catalog-back" href={from}>← {modelBackLabel(from)}</a><div className="catalog-detail-head"><div><h2>{display(model)}</h2>{display(model) !== name(model) && <small>{name(model)}</small>}</div><span className={enabled(model) ? "catalog-badge ready" : "catalog-badge"}>{enabled(model) ? "Habilitado" : "Deshabilitado"}</span></div><div className="catalog-info-grid"><article><h3>Proveedor</h3><p>{provider ? <a href={url("providers", name(provider), "modelos")}>{display(provider)}</a> : "Sin proveedor"}</p></article><article><h3>ID en el proveedor</h3><p>{text(spec(model).providerModelId) || "—"}</p></article><article><h3>Capacidades</h3><p>{capabilities(model)}</p></article><article><h3>Destinos configurados</h3><p>{destinations.length}</p></article></div><div className="catalog-info-grid catalog-followup"><article><h3>Rendimiento observado</h3><p>{usage ? `${usage.requests} solicitudes con uso · ${usage.avgLatencyMillis == null ? "Latencia sin datos" : `${Math.round(usage.avgLatencyMillis)} ms de latencia media`} · ${usage.outputTokensPerSecond == null ? "Velocidad sin datos" : `${usage.outputTokensPerSecond.toLocaleString("es", { maximumFractionDigits: 1 })} tokens/s de salida`}` : usageError ? "No pudimos cargar las métricas del modelo." : "Sin solicitudes con uso reportado para este modelo."}</p><p>La velocidad se calcula sobre la duración completa de las solicitudes; uptime por sondeos aún no está disponible.</p><a href="#/data">Ver todas las métricas →</a></article><article><h3>Claves compatibles</h3><p>El modelo y la clave se vinculan mediante destinos. Hay {destinations.length} {destinations.length === 1 ? "destino configurado" : "destinos configurados"}.</p></article></div><form className="catalog-pricing" onSubmit={savePricing} key={`${id}-${String(pricing.inputPerMillion)}-${String(pricing.outputPerMillion)}`}><div><h3>Tarifa para estimaciones</h3><p>No es una factura. Se aplica a tokens registrados, usando la tarifa actual incluso para solicitudes antiguas.</p></div>{priceError && <div className="form-error" role="alert">{priceError}</div>}{priceMessage && <div className="notice" role="status">{priceMessage}</div>}<label className="catalog-pricing-toggle"><input type="checkbox" name="configured" defaultChecked={pricing.currency === "USD"}/> Tengo una tarifa para este modelo</label><div className="catalog-form-grid"><label>Entrada · USD por 1 M tokens<input name="inputPrice" type="number" min="0" max="1000000" step="any" defaultValue={typeof pricing.inputPerMillion === "number" ? pricing.inputPerMillion : ""} placeholder="0 para gratuito"/></label><label>Salida · USD por 1 M tokens<input name="outputPrice" type="number" min="0" max="1000000" step="any" defaultValue={typeof pricing.outputPerMillion === "number" ? pricing.outputPerMillion : ""} placeholder="0 para gratuito"/></label></div><button disabled={savingPrice}>{savingPrice ? "Guardando…" : "Guardar tarifa"}</button></form></>;
}

function RouteDetail({ id, data, stepsFor, onSaved }: { id: string; data: CatalogData; stepsFor: (route: Resource) => Resource[]; onSaved: () => Promise<void> }) {
  const [editing, setEditing] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [activeStep, setActiveStep] = useState("");
  const [candidate, setCandidate] = useState("");
  const [newModel, setNewModel] = useState("");
  const [newKey, setNewKey] = useState("");
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  const route = find(data.routes, id);
  if (!route) return <><a className="catalog-back" href={url("routes")}>← Rutas</a><Empty title="Ruta no encontrada" message="Puede haber sido eliminada desde otra sesión."/></>;
  const strategy = find(data.strategies, ref(spec(route).strategyRef));
  const steps = stepsFor(route);
  const description = (destination: Resource) => {
    const model = find(data.models, ref(spec(destination).modelRef));
    const key = find(data.credentials, ref(spec(destination).credentialRef));
    const connection = find(data["provider-connections"], ref(spec(model ?? destination).connectionRef));
    const provider = find(data.providers, ref(spec(connection ?? destination).providerRef));
    const egress = find(data.egresses, ref(spec(key ?? destination).egressRef));
    return { model: model ? display(model) : ref(spec(destination).modelRef) || name(destination), key: key ? display(key) : ref(spec(destination).credentialRef) || "sin clave", provider: provider ? display(provider) : "Proveedor sin identificar", egress: egress ? display(egress) : "Sin identificar" };
  };
  const available = data.destinations.filter(item => !selected.includes(name(item)));
  const providerOfModel = (model: Resource) => ref(spec(find(data["provider-connections"], ref(spec(model).connectionRef)) ?? model).providerRef);
  const providerOfKey = (key: Resource) => ref(spec(find(data["provider-accounts"], ref(spec(key).providerAccountRef)) ?? key).providerRef);
  const compatibleKeys = data.credentials.filter(key => newModel && providerOfKey(key) === providerOfModel(find(data.models, newModel) ?? key));
  function beginEdit() { setSelected(steps.map(name)); setActiveStep(steps[0] ? name(steps[0]) : ""); setCandidate(""); setMessage(""); setEditing(true); }
  function move(index: number, direction: number) { setSelected(current => { const copy = [...current]; [copy[index], copy[index + direction]] = [copy[index + direction], copy[index]]; return copy; }); }
  async function createOption() {
    if (!newModel || !newKey || selected.length >= 32) return;
    const optionName = `option-${crypto.randomUUID().slice(0, 8)}`;
    setBusy(true); setMessage("");
    try {
      await api.createResource("destinations", { kind: "Destination", state: "present", metadata: { name: optionName }, spec: { modelRef: { name: newModel }, credentialRef: { name: newKey }, weight: 100, enabled: true } } as Resource);
      setSelected(current => [...current, optionName]); setActiveStep(optionName); setNewModel(""); setNewKey("");
      await onSaved();
      setMessage("Opción creada y añadida al borrador. Guarda el recorrido y luego publícalo para activarla.");
    } catch { setMessage("No se pudo crear la opción. Comprueba que modelo y clave pertenezcan al mismo proveedor."); }
    finally { setBusy(false); }
  }
  async function save() {
    if (!strategy || !selected.length) return;
    setBusy(true); setMessage("");
    try {
      await api.updateResource("strategies", name(strategy), Number(object(strategy.metadata).resourceVersion), { kind: "Strategy", state: "present", metadata: { name: name(strategy), displayName: object(strategy.metadata).displayName, description: object(strategy.metadata).description }, spec: { ...spec(strategy), destinations: selected.map(value => ({ name: value })), maxAttempts: selected.length } } as Resource);
      await onSaved(); setEditing(false); setMessage("Recorrido guardado como borrador. Pulsa Publicar para activarlo.");
    } catch { setMessage("No se pudo guardar el recorrido. Comprueba si cambió en otra sesión."); }
    finally { setBusy(false); }
  }
  async function publish() {
    if (!strategy || !confirm("¿Publicar este recorrido? Las nuevas solicitudes usarán la versión guardada.")) return;
    setBusy(true); setMessage("");
    try { await api.publishStrategy(name(strategy), Number(object(strategy.metadata).resourceVersion)); await onSaved(); setMessage("Recorrido publicado. Las nuevas solicitudes usarán esta versión."); }
    catch { setMessage("No se pudo publicar. Actualiza la página y vuelve a intentarlo."); }
    finally { setBusy(false); }
  }
  const shown = editing ? selected.map(value => find(data.destinations, value)).filter((item): item is Resource => Boolean(item)) : steps;
  const active = shown.find(item => name(item) === activeStep) ?? shown[0];
  const activeIndex = active ? shown.findIndex(item => name(item) === name(active)) : -1;
  const detail = active ? description(active) : null;
  const flow: FlowStep[] = shown.map(item => {
    const info = description(item);
    return { id: name(item), provider: info.provider, model: info.model, credential: info.key };
  });
  const alias = text(spec(route).modelAlias) || display(route);
  return <>
    <a className="catalog-back" href={url("routes")}>← Rutas</a>
    <div className="route-page-head"><div><p className="eyebrow">RECORRIDO DE RESPALDO</p><h1 id="catalog-title">Ruta {alias}</h1><p>ModelCairn prueba los destinos en orden. El tráfico usa la última versión publicada.</p></div><span className={enabled(route) ? "catalog-badge ready" : "catalog-badge"}>{enabled(route) ? "Habilitada" : "Deshabilitada"}</span></div>
    <div className="catalog-route-toolbar"><strong>{shown.length === 1 ? "Un destino" : shown.length + " destinos en secuencia"}</strong><div>{strategy && !editing && <button type="button" className="secondary" onClick={beginEdit}>Editar recorrido</button>}{strategy && !editing && <button type="button" onClick={publish} disabled={busy}>Publicar borrador guardado</button>}{editing && <button type="button" className="secondary" onClick={() => setEditing(false)} disabled={busy}>Cancelar</button>}{editing && <button type="button" onClick={save} disabled={busy || !selected.length}>{busy ? "Guardando…" : "Guardar borrador"}</button>}</div></div>
    {message && <div className={message.startsWith("No se") ? "form-error" : "notice"} role="status">{message}</div>}
    <div className="route-workspace">
      <section aria-label="Recorrido"><RouteFlow alias={alias} steps={flow} selected={active ? name(active) : ""} onSelect={setActiveStep}/>{editing && <div className="route-canvas-actions"><span>Los cambios se activan después de guardar y publicar.</span></div>}</section>
      <section className="route-inspector" aria-label="Detalles del destino">
        <h3>{activeIndex < 1 ? "Destino principal" : "Respaldo " + activeIndex}</h3>
        {detail ? <><p>Selecciona otra tarjeta del recorrido para ver su configuración.</p><dl><div><dt>Proveedor</dt><dd>{detail.provider}</dd></div><div><dt>Modelo</dt><dd>{detail.model}</dd></div><div><dt>Clave API</dt><dd>{detail.key}</dd></div><div><dt>Salida de red</dt><dd>{detail.egress}</dd></div></dl></> : <p>Esta ruta no tiene destinos configurados.</p>}
        {editing && active && <div className="route-inspector-actions"><button type="button" disabled={activeIndex === 0} onClick={() => move(activeIndex, -1)}>↑ Subir</button><button type="button" disabled={activeIndex === selected.length - 1} onClick={() => move(activeIndex, 1)}>↓ Bajar</button><button type="button" disabled={selected.length === 1} onClick={() => { setSelected(current => current.filter(value => value !== name(active))); setActiveStep(""); }}>Quitar</button></div>}
        {editing && <div className="route-add-panel"><h3>Añadir respaldo</h3><label>Modelo<select value={newModel} onChange={event => { setNewModel(event.target.value); setNewKey(""); }}><option value="">Selecciona un modelo</option>{data.models.filter(enabled).map(item => <option key={name(item)} value={name(item)}>{display(item)}</option>)}</select></label><label>Clave API<select value={newKey} onChange={event => setNewKey(event.target.value)} disabled={!newModel}><option value="">Selecciona una clave compatible</option>{compatibleKeys.filter(enabled).map(item => <option key={name(item)} value={name(item)}>{display(item)}</option>)}</select></label>{newModel && !compatibleKeys.length && <p>Vincula una clave a este proveedor antes de añadirlo.</p>}<button type="button" disabled={busy || !newModel || !newKey || selected.length >= 32} onClick={createOption}>Crear y añadir</button><label>Opción ya configurada<select value={candidate} onChange={event => setCandidate(event.target.value)}><option value="">Selecciona otra opción</option>{available.map(item => <option key={name(item)} value={name(item)}>{description(item).model} · {description(item).key}</option>)}</select></label><button type="button" className="secondary" disabled={!candidate} onClick={() => { setSelected(current => [...current, candidate]); setActiveStep(candidate); setCandidate(""); }}>Añadir existente</button></div>}
      </section>
    </div>
    <div className="catalog-info-grid catalog-followup"><article><h3>Intentos máximos del borrador</h3><p>{typeof spec(strategy ?? route).maxAttempts === "number" ? String(spec(strategy ?? route).maxAttempts) : "No especificados"}</p></article><article><h3>Acceso de aplicaciones</h3><p>Para usar el alias, una aplicación necesita un token autorizado para esta ruta. <a href="#/tokens">Ver aplicaciones</a>.</p></article></div>
  </>;
}
