import { useEffect, useState } from "react";
import { api, APIError, Resource } from "./api/client";

const kinds = [
  ["providers", "Proveedores"], ["provider-accounts", "Cuentas"], ["provider-connections", "Conexiones"],
  ["credentials", "Credenciales"], ["egresses", "Egresos"], ["models", "Modelos"],
  ["destinations", "Destinos"], ["strategies", "Estrategias"], ["routes", "Rutas"], ["agent-tokens", "Agentes"],
] as const;

export default function Resources() {
  const [kind, setKind] = useState("providers");
  const [items, setItems] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = () => {
    setLoading(true); setError("");
    api.listResources(kind).then((page) => setItems(page.items))
      .catch((reason) => setError(reason instanceof APIError ? reason.code : "request_failed"))
      .finally(() => setLoading(false));
  };
  useEffect(load, [kind]);
  async function remove(item: Resource) {
    const metadata = item.metadata as { name?: string; resourceVersion?: number };
    if (!metadata.name || !metadata.resourceVersion || !confirm(`¿Eliminar ${metadata.name}? Las referencias activas impedirán una eliminación insegura.`)) return;
    try { await api.deleteResource(kind, metadata.name, metadata.resourceVersion); load(); }
    catch (reason) { setError(reason instanceof APIError && reason.code === "graph_conflict" ? "Este recurso todavía está siendo utilizado por otro." : "No se pudo eliminar el recurso. Actualiza la lista e inténtalo otra vez."); }
  }
  return <section aria-labelledby="resources-title">
    <p className="eyebrow">CONFIGURACIÓN</p><h1 id="resources-title">Recursos</h1>
    <p className="lede">Explora la configuración efectiva. Las relaciones y versiones se validan en el servidor.</p>
    <div className="kind-tabs" role="tablist" aria-label="Tipos de recurso">{kinds.map(([value,label]) => <button role="tab" aria-selected={kind===value} key={value} onClick={() => setKind(value)}>{label}</button>)}</div>
    {error && <div className="form-error" role="alert">{error}</div>}
    {loading ? <div className="loading" role="status"><span className="spinner"/>Cargando recursos…</div> : items.length === 0 ? <div className="list-empty">No hay recursos de este tipo.</div> : <div className="resource-list">{items.map((item) => {
      const metadata = item.metadata as { name:string; displayName?:string; resourceVersion:number };
      return <article key={metadata.name}><div><h2>{metadata.displayName ?? metadata.name}</h2><p><code>{metadata.name}</code> · versión {metadata.resourceVersion}</p></div><span className="resource-kind">{item.kind}</span><details><summary>Ver configuración</summary><pre>{JSON.stringify("spec" in item ? item.spec : {}, null, 2)}</pre></details><button className="danger" onClick={() => remove(item)}>Eliminar</button></article>;
    })}</div>}
  </section>;
}
