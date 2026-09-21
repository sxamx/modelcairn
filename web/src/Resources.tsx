import { FormEvent, useEffect, useState } from "react";
import { api, APIError, Resource } from "./api/client";

const kinds = [
  ["providers", "Proveedores", "Provider"], ["provider-accounts", "Cuentas", "ProviderAccount"], ["provider-connections", "Conexiones", "ProviderConnection"],
  ["credentials", "Credenciales", "Credential"], ["egresses", "Egresos", "Egress"], ["models", "Modelos", "Model"],
  ["destinations", "Destinos", "Destination"], ["strategies", "Estrategias", "Strategy"], ["routes", "Rutas", "Route"], ["agent-tokens", "Agentes", "AgentToken"],
] as const;

type Editable = { name: string; displayName: string; description: string; spec: string; version?: number };
const empty = (): Editable => ({ name: "", displayName: "", description: "", spec: "{}" });
export type ResourceArea = "providers" | "models" | "routes" | "all";
const areaKinds: Record<ResourceArea, string[]> = {
  providers: ["providers", "provider-accounts", "provider-connections", "credentials", "egresses", "destinations"],
  models: ["models"],
  routes: ["routes", "strategies", "destinations"],
  all: kinds.map(([value]) => value),
};
const areaNames: Record<ResourceArea, { title: string; description: string }> = {
  providers: { title: "Proveedores", description: "Gestiona los proveedores y las conexiones que permiten llegar a sus modelos." },
  models: { title: "Modelos", description: "Modelos configurados para tus proveedores. La disponibilidad puede variar entre claves." },
  routes: { title: "Rutas", description: "Aliases que usan tus aplicaciones y estrategias de selección y respaldo." },
  all: { title: "Recursos", description: "Configuración avanzada de todos los recursos del gateway." },
};
function kindFromHash(area: ResourceArea): string {
  const fromUrl = window.location.hash.slice(2).split("/")[1];
  return areaKinds[area].includes(fromUrl) ? fromUrl : areaKinds[area][0];
}

export default function Resources({ area = "all", onOpenWizard }: { area?: ResourceArea; onOpenWizard?: () => void }) {
  const [kind, setKind] = useState(() => kindFromHash(area)); const [items, setItems] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(true); const [error, setError] = useState(""); const [editor, setEditor] = useState<Editable | null>(null); const [busy, setBusy] = useState(false);
  const selected = kinds.find(([value]) => value === kind)!;
  const visibleKinds = kinds.filter(([value]) => areaKinds[area].includes(value));
  const load = () => { setLoading(true); setError(""); api.listResources(kind).then((page) => setItems(page.items)).catch((reason) => setError(reason instanceof APIError ? reason.code : "request_failed")).finally(() => setLoading(false)); };
  useEffect(() => { setEditor(null); load(); }, [kind]);
  useEffect(() => {
    const sync = () => setKind(kindFromHash(area));
    sync();
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, [area]);
  function selectKind(value: string) {
    if (kind === value) return;
    window.location.hash = `/${area}/${value}`;
    setKind(value);
  }
  function edit(item: Resource) { const metadata = item.metadata as { name:string; displayName?:string; description?:string; resourceVersion:number }; setEditor({ name:metadata.name, displayName:metadata.displayName ?? "", description:metadata.description ?? "", version:metadata.resourceVersion, spec:JSON.stringify("spec" in item ? item.spec : {}, null, 2) }); }
  async function save(event: FormEvent) {
    event.preventDefault(); if (!editor) return; setBusy(true); setError("");
    try {
      const spec = JSON.parse(editor.spec) as Record<string, unknown>;
      const metadata: Record<string, unknown> = { name:editor.name }; if (editor.displayName) metadata.displayName=editor.displayName; if (editor.description) metadata.description=editor.description;
      const resource = { kind:selected[2], state:"present", metadata, spec } as Resource;
      if (editor.version) await api.updateResource(kind, editor.name, editor.version, resource); else await api.createResource(kind, resource);
      setEditor(null); load();
    } catch (reason) { setError(reason instanceof SyntaxError ? "La configuración JSON no es válida." : reason instanceof APIError && reason.status === 412 ? "El recurso cambió en otra sesión. Actualiza la lista antes de guardar." : "El servidor rechazó el recurso. Revisa campos y referencias."); }
    finally { setBusy(false); }
  }
  async function remove(item: Resource) { const metadata=item.metadata as {name?:string;resourceVersion?:number}; if(!metadata.name||!metadata.resourceVersion||!confirm(`¿Eliminar ${metadata.name}? Las referencias activas impedirán una eliminación insegura.`))return; try{await api.deleteResource(kind,metadata.name,metadata.resourceVersion);load();}catch(reason){setError(reason instanceof APIError&&reason.code==="graph_conflict"?"Este recurso todavía está siendo utilizado por otro.":"No se pudo eliminar el recurso. Actualiza la lista e inténtalo otra vez.");} }
  async function publish(item: Resource) { const metadata=item.metadata as {name:string;resourceVersion:number}; if(!confirm(`¿Publicar la versión actual de ${metadata.name}? Las rutas asociadas comenzarán a usarla.`))return; try{await api.publishStrategy(metadata.name,metadata.resourceVersion);load();}catch{setError("No se pudo publicar. Actualiza la lista y vuelve a intentarlo.");} }
  return <section aria-labelledby="resources-title"><h1 id="resources-title">{areaNames[area].title}</h1><p className="lede">{areaNames[area].description}</p>
    {area === "routes" && onOpenWizard && <div className="resource-quickstart"><div><strong>¿Quieres preparar una ruta completa?</strong><p>El asistente crea proveedor, clave, modelo, ruta y acceso API en orden.</p></div><button className="secondary" onClick={onOpenWizard}>Abrir asistente</button></div>}
    {visibleKinds.length > 1 && <div className="kind-tabs" role="tablist" aria-label="Tipos de recurso">{visibleKinds.map(([value,label])=><button role="tab" aria-selected={kind===value} key={value} onClick={()=>selectKind(value)}>{label}</button>)}</div>}
    <button className="secondary resource-create" onClick={()=>setEditor(empty())}>Crear {selected[1].toLowerCase()}</button>
    {error&&<div className="form-error" role="alert">{error}</div>}{editor&&<form className="resource-editor" onSubmit={save}><div><p className="step">{editor.version?"EDITAR":"NUEVO RECURSO"}</p><h2>{editor.version?editor.name:selected[1]}</h2></div><label>Nombre<input value={editor.name} required pattern="[a-z][a-z0-9-]{0,62}" disabled={Boolean(editor.version)} onChange={e=>setEditor({...editor,name:e.target.value})}/></label><label>Nombre visible<input value={editor.displayName} maxLength={120} onChange={e=>setEditor({...editor,displayName:e.target.value})}/></label><label>Descripción<textarea value={editor.description} maxLength={2000} onChange={e=>setEditor({...editor,description:e.target.value})}/></label><label>Configuración JSON<textarea className="spec-editor" spellCheck={false} value={editor.spec} required onChange={e=>setEditor({...editor,spec:e.target.value})}/><small>Modo avanzado: el servidor comprobará campos, referencias y seguridad antes de guardar.</small></label><div className="editor-actions"><button type="button" className="secondary" onClick={()=>setEditor(null)}>Cancelar</button><button disabled={busy}>{busy?"Guardando…":"Guardar recurso"}</button></div></form>}
    {loading?<div className="loading" role="status"><span className="spinner"/>Cargando recursos…</div>:items.length===0?<div className="list-empty">No hay recursos de este tipo.</div>:<div className="resource-list">{items.map(item=>{const metadata=item.metadata as {name:string;displayName?:string;resourceVersion:number};return <article key={metadata.name}><div><h2>{metadata.displayName??metadata.name}</h2><p><code>{metadata.name}</code> · versión {metadata.resourceVersion}</p></div><span className="resource-kind">{item.kind}</span><details><summary>Ver configuración</summary><pre>{JSON.stringify("spec" in item?item.spec:{},null,2)}</pre></details><div className="resource-actions"><button className="secondary" onClick={()=>edit(item)}>Editar</button>{kind==="strategies"&&<button className="publish" onClick={()=>publish(item)}>Publicar</button>}<button className="danger" onClick={()=>remove(item)}>Eliminar</button></div></article>;})}</div>}
  </section>;
}
