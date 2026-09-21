import { FormEvent, useCallback, useEffect, useState } from "react";
import { api, APIError, Resource } from "./api/client";
import Resources, { ResourceArea } from "./Resources";

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

export default function Catalog({ area, onOpenWizard, onOpenSecrets }: { area: Area; onOpenWizard: () => void; onOpenSecrets: () => void }) {
  const [data, setData] = useState<CatalogData>(blank);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [location, setLocation] = useState(window.location.hash);
  const [creating, setCreating] = useState(false);
  const [saving, setSaving] = useState(false);
  const [createError, setCreateError] = useState("");
  const refresh = useCallback(async () => {
    setLoading(true); setError("");
    try {
      const pages = await Promise.all(kinds.map(listAll));
      setData(Object.fromEntries(kinds.map((kind, index) => [kind, pages[index]])) as CatalogData);
    } catch { setError("No pudimos cargar la configuración. Revisa la conexión e inténtalo de nuevo."); }
    finally { setLoading(false); }
  }, []);
  useEffect(() => { void refresh(); }, [refresh]);
  useEffect(() => {
    const sync = () => { setLocation(window.location.hash); setCreating(false); };
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);
  const parts = location.slice(2).split("/");
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
  const advanced = id === "advanced";
  const intro = area === "providers" ? "Tus servicios de IA, sus modelos y las claves vinculadas, agrupados en un solo lugar."
    : area === "models" ? "Catálogo de modelos configurados. Cada modelo pertenece a una conexión de proveedor."
    : "Aliases para tus aplicaciones y orden de respaldo entre destinos.";
  return <section className="catalog" aria-labelledby="catalog-title">
    {advanced ? <><a className="catalog-back" href={url(area)}>← Volver a {title[area].toLowerCase()}</a><div className="catalog-advanced-note"><strong>Modo avanzado</strong><p>Aquí se editan las entidades internas y sus referencias. Los cambios se validan en el servidor; vuelve a la vista principal para navegar por proveedor, modelo o ruta.</p></div><Resources key={area} area={area} onOpenWizard={onOpenWizard}/></> :
      <>
        <div className="catalog-heading"><div><h1 id="catalog-title">{title[area]}</h1><p className="lede">{intro}</p></div><div className="catalog-heading-actions">{area === "providers" && !id && <button onClick={() => { setCreateError(""); setCreating(true); }}>Añadir proveedor</button>}{area === "routes" && !id && <button onClick={onOpenWizard}>Crear con asistente</button>}</div></div>
        {error && <div className="form-error" role="alert">{error} <button className="catalog-inline-button" onClick={() => void refresh()}>Reintentar</button></div>}
        {loading ? <div className="loading" role="status"><span className="spinner"/>Cargando configuración…</div> :
          area === "providers" ? id ? <ProviderDetail id={id} tab={tab} data={data} models={modelsFor(id)} keys={keysFor(id)} onOpenSecrets={onOpenSecrets}/> :
            <><div className="catalog-grid">{data.providers.map(provider => {
              const pid = name(provider), models = modelsFor(pid), keys = keysFor(pid);
              const connections = data["provider-connections"].filter(row => ref(spec(row).providerRef) === pid);
              return <a className="catalog-card" key={pid} href={url("providers", pid)}><div className="catalog-card-top"><strong>{display(provider)}</strong><span className={connections.length && keys.length && models.length ? "catalog-badge ready" : "catalog-badge"}>{connections.length && keys.length && models.length ? "Configurado" : "Pendiente"}</span></div><p>{pid}</p><div className="catalog-card-stats"><span>{keys.length} {keys.length === 1 ? "clave" : "claves"}</span><span>{models.length} {models.length === 1 ? "modelo" : "modelos"}</span></div></a>;
            })}</div>{!data.providers.length && <Empty title="Aún no hay proveedores" message="Añade un proveedor para comenzar; puedes conectar sus modelos y claves después."/>}</> :
          area === "models" ? id ? <ModelDetail id={id} data={data} providerForModel={providerForModel} destinationsFor={destinationsFor}/> :
            <><div className="catalog-table-wrap"><table><thead><tr><th>Modelo</th><th>Proveedor</th><th>Capacidades</th><th>Destinos configurados</th></tr></thead><tbody>{data.models.map(row => <tr key={name(row)}><td><a href={url("models", name(row))}>{display(row)}</a></td><td>{display(find(data.providers, providerForModel(row)) ?? row)}</td><td>{capabilities(row)}</td><td>{destinationsFor(name(row)).length}</td></tr>)}</tbody></table></div>{!data.models.length && <Empty title="Aún no hay modelos" message="Los modelos que configures en cada proveedor aparecerán aquí."/>}</> :
          id ? <RouteDetail id={id} data={data} stepsFor={stepsFor}/> :
            <><div className="catalog-table-wrap"><table><thead><tr><th>Alias</th><th>Recorrido configurado</th><th>Estado de configuración</th></tr></thead><tbody>{data.routes.map(row => {
              const steps = stepsFor(row), chain = steps.map(step => display(find(data.models, ref(spec(step).modelRef)) ?? step)).join(" → ");
              return <tr key={name(row)}><td><a href={url("routes", name(row))}>{text(spec(row).modelAlias) || display(row)}</a></td><td>{chain || "Sin destinos"}</td><td><span className={enabled(row) ? "catalog-badge ready" : "catalog-badge"}>{enabled(row) ? "Habilitada" : "Deshabilitada"}</span></td></tr>;
            })}</tbody></table></div>{!data.routes.length && <Empty title="Aún no hay rutas" message="Puedes usar el asistente para crear una ruta completa."/>}</>
        }
        {!loading && <div className="catalog-footer"><a href={url(area, "advanced")}>Opciones avanzadas</a><span>Para editar relaciones técnicas o configuraciones JSON.</span></div>}
        {creating && <div className="catalog-dialog-backdrop" onMouseDown={event => { if (event.target === event.currentTarget) setCreating(false); }}><form className="catalog-dialog" onSubmit={createProvider} role="dialog" aria-modal="true" aria-labelledby="new-provider-title"><div className="catalog-dialog-head"><h2 id="new-provider-title">Añadir proveedor</h2><button type="button" aria-label="Cerrar" onClick={() => setCreating(false)}>×</button></div><p>Primero crea el proveedor. Después podrás añadir sus conexiones, modelos y claves.</p>{createError && <div className="form-error" role="alert">{createError}</div>}<label>Identificador técnico<input name="name" required pattern="[a-z][a-z0-9-]{0,62}" placeholder="mi-proveedor"/></label><label>Nombre visible<input name="displayName" maxLength={120} placeholder="Mi proveedor"/></label><div className="catalog-dialog-actions"><button type="button" className="secondary" onClick={() => setCreating(false)}>Cancelar</button><button disabled={saving}>{saving ? "Creando…" : "Crear proveedor"}</button></div></form></div>}
      </>}
  </section>;
}

function Empty({ title, message }: { title: string; message: string }) { return <div className="catalog-empty"><h2>{title}</h2><p>{message}</p></div>; }

function ProviderDetail({ id, tab, data, models, keys, onOpenSecrets }: { id: string; tab: string; data: CatalogData; models: Resource[]; keys: Resource[]; onOpenSecrets: () => void }) {
  const provider = find(data.providers, id);
  if (!provider) return <><a className="catalog-back" href={url("providers")}>← Proveedores</a><Empty title="Proveedor no encontrado" message="Puede haber sido eliminado desde otra sesión."/></>;
  const connections = data["provider-connections"].filter(row => ref(spec(row).providerRef) === id);
  const tabs = [["resumen","Resumen"],["modelos","Modelos"],["claves","API keys"],["configuracion","Configuración"]];
  return <><a className="catalog-back" href={url("providers")}>← Proveedores</a><div className="catalog-detail-head"><h2>{display(provider)}</h2><span className={connections.length && keys.length && models.length ? "catalog-badge ready" : "catalog-badge"}>{connections.length && keys.length && models.length ? "Configurado" : "Pendiente"}</span></div><div className="catalog-tabs" role="tablist" aria-label="Detalles del proveedor">{tabs.map(([key,label]) => <a role="tab" aria-selected={tab === key} key={key} href={url("providers", id, key)}>{label}</a>)}</div>
    {tab === "modelos" ? <><div className="catalog-section-head"><span>{models.length} {models.length === 1 ? "modelo configurado" : "modelos configurados"}</span><a href={url("models")}>Ver catálogo completo</a></div><div className="catalog-table-wrap"><table><thead><tr><th>Modelo</th><th>Identificador en el proveedor</th><th>Capacidades</th></tr></thead><tbody>{models.map(row => <tr key={name(row)}><td><a href={url("models", name(row))}>{display(row)}</a></td><td>{text(spec(row).providerModelId)}</td><td>{capabilities(row)}</td></tr>)}</tbody></table></div>{!models.length && <Empty title="Sin modelos" message="Agrega modelos desde Opciones avanzadas; la detección automática aún no está disponible."/>}</> :
    tab === "claves" ? <><div className="catalog-section-head"><span>{keys.length} {keys.length === 1 ? "clave vinculada" : "claves vinculadas"}</span><button onClick={onOpenSecrets}>Claves guardadas</button></div><div className="catalog-table-wrap"><table><thead><tr><th>Clave vinculada</th><th>Secreto guardado</th><th>Salida de red</th><th>Estado</th></tr></thead><tbody>{keys.map(row => <tr key={name(row)}><td>{display(row)}</td><td>{ref(spec(row).secretRef) || "—"}</td><td>{ref(spec(row).egressRef) || "—"}</td><td><span className={enabled(row) ? "catalog-badge ready" : "catalog-badge"}>{enabled(row) ? "Habilitada" : "Deshabilitada"}</span></td></tr>)}</tbody></table></div>{!keys.length && <Empty title="Sin claves vinculadas" message="Una clave guardada se vincula a este proveedor mediante una credencial. Usa Opciones avanzadas para completar la relación."/>}</> :
    tab === "configuracion" ? <div className="catalog-info-grid"><article><h3>Identificador</h3><p>{name(provider)}</p></article><article><h3>Conexiones API</h3>{connections.length ? connections.map(row => <p key={name(row)}>{text(spec(row).baseUrl) || name(row)}</p>) : <p>Sin conexiones</p>}</article><article><h3>Descripción</h3><p>{text(object(provider.metadata).description) || "Sin descripción"}</p></article></div> :
    <div className="catalog-summary"><article><h3>Modelos</h3><strong>{models.length}</strong><p>Configurados en este proveedor</p></article><article><h3>API keys</h3><strong>{keys.length}</strong><p>Vinculadas a sus cuentas</p></article><article><h3>Conexiones</h3><strong>{connections.length}</strong><p>URLs de API configuradas</p></article><article><h3>Disponibilidad</h3><strong>Sin datos</strong><p>Requiere comprobaciones periódicas</p></article></div>}
  </>;
}

function ModelDetail({ id, data, providerForModel, destinationsFor }: { id: string; data: CatalogData; providerForModel: (row: Resource) => string; destinationsFor: (id: string) => Resource[] }) {
  const model = find(data.models, id);
  if (!model) return <><a className="catalog-back" href={url("models")}>← Modelos</a><Empty title="Modelo no encontrado" message="Puede haber sido eliminado desde otra sesión."/></>;
  const provider = find(data.providers, providerForModel(model));
  const destinations = destinationsFor(id);
  return <><a className="catalog-back" href={url("models")}>← Modelos</a><div className="catalog-detail-head"><h2>{display(model)}</h2><span className={enabled(model) ? "catalog-badge ready" : "catalog-badge"}>{enabled(model) ? "Habilitado" : "Deshabilitado"}</span></div><div className="catalog-info-grid"><article><h3>Proveedor</h3><p>{provider ? <a href={url("providers", name(provider))}>{display(provider)}</a> : "Sin proveedor"}</p></article><article><h3>Identificador de modelo</h3><p>{text(spec(model).providerModelId) || "—"}</p></article><article><h3>Capacidades declaradas</h3><p>{capabilities(model)}</p></article><article><h3>Destinos configurados</h3><p>{destinations.length}</p></article></div><div className="catalog-info-grid catalog-followup"><article><h3>Latencia, tokens/s y uptime</h3><p>Sin datos agregados por modelo todavía. No se muestran valores simulados.</p></article><article><h3>Claves compatibles</h3><p>La configuración actual vincula modelo y clave mediante destinos. Hay {destinations.length} {destinations.length === 1 ? "destino configurado" : "destinos configurados"}.</p></article></div></>;
}

function RouteDetail({ id, data, stepsFor }: { id: string; data: CatalogData; stepsFor: (route: Resource) => Resource[] }) {
  const route = find(data.routes, id);
  if (!route) return <><a className="catalog-back" href={url("routes")}>← Rutas</a><Empty title="Ruta no encontrada" message="Puede haber sido eliminada desde otra sesión."/></>;
  const strategy = find(data.strategies, ref(spec(route).strategyRef));
  const steps = stepsFor(route);
  return <><a className="catalog-back" href={url("routes")}>← Rutas</a><div className="catalog-detail-head"><h2>{text(spec(route).modelAlias) || display(route)}</h2><span className={enabled(route) ? "catalog-badge ready" : "catalog-badge"}>{enabled(route) ? "Habilitada" : "Deshabilitada"}</span></div><p className="catalog-detail-sub">Esta es la configuración guardada. Las solicitudes usan la última versión publicada de la estrategia, que puede diferir.</p><div className="catalog-route-flow">{steps.map((step, index) => {
    const model = find(data.models, ref(spec(step).modelRef)), key = find(data.credentials, ref(spec(step).credentialRef));
    return <div className="catalog-route-step" key={name(step)}><span className="catalog-step-number">{index + 1}</span><div><strong>{model ? display(model) : ref(spec(step).modelRef) || name(step)}</strong><p>Clave: {key ? display(key) : ref(spec(step).credentialRef) || "sin vincular"}</p></div></div>;
  })}{!steps.length && <Empty title="Sin destinos" message="La estrategia de esta ruta todavía no contiene pasos disponibles."/>}</div><div className="catalog-info-grid catalog-followup"><article><h3>Intentos máximos</h3><p>{typeof spec(strategy ?? route).maxAttempts === "number" ? String(spec(strategy ?? route).maxAttempts) : "No especificados"}</p></article><article><h3>Identificador interno</h3><p>{name(route)}</p></article></div></>;
}
