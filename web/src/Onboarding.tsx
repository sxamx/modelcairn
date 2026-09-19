import { FormEvent, KeyboardEvent, useEffect, useRef, useState } from "react";
import { api, APIError, Configuration, ConfigurationPlan } from "./api/client";
import { buildOnboardingConfiguration } from "./onboarding-configuration";

type Props = { onClose: () => void; onComplete: () => void };
type Result = { token: string; alias: string } | null;
type Pending = { configuration: Configuration; plan: ConfigurationPlan; secretName: string; secret: string; agentName: string; alias: string; configured: boolean; tokenRecovery: boolean } | null;

export default function Onboarding({ onClose, onComplete }: Props) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [result, setResult] = useState<Result>(null);
  const [pending, setPending] = useState<Pending>(null);
  const dialog = useRef<HTMLElement>(null);
  useEffect(() => {
    const previous=document.activeElement as HTMLElement|null;
    const target=document.querySelector<HTMLElement>('[role="dialog"]');
    target?.setAttribute("tabindex","-1"); target?.focus();
    const keys=(event:globalThis.KeyboardEvent)=>{if(event.key==="Escape"){onClose();return;}if(event.key!=="Tab"||!target)return;const controls=[...target.querySelectorAll<HTMLElement>('button,input,textarea,select')].filter(item=>!item.hasAttribute("disabled"));if(!controls.length)return;const first=controls[0],last=controls[controls.length-1];if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus();}};
    target?.addEventListener("keydown",keys);
    return()=>{target?.removeEventListener("keydown",keys);previous?.focus();};
  }, [onClose, pending, result]);
  function dialogKeys(event:KeyboardEvent<HTMLElement>) { if(event.key==="Escape"&&!busy){onClose();return;} if(event.key!=="Tab")return; const controls=[...event.currentTarget.querySelectorAll<HTMLElement>('button,input,textarea,select,[tabindex]:not([tabindex="-1"])')].filter(item=>!item.hasAttribute("disabled")); if(!controls.length)return; const first=controls[0],last=controls[controls.length-1]; if(event.shiftKey&&document.activeElement===first){event.preventDefault();last.focus();}else if(!event.shiftKey&&document.activeElement===last){event.preventDefault();first.focus();} }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = event.currentTarget;
    const values = new FormData(form);
    const name = String(values.get("name"));
    const secretName = `${name}-secret`;
    const agentName = String(values.get("agent"));
    const alias = String(values.get("alias"));
    const secret = String(values.get("secret"));
    const configuration = buildOnboardingConfiguration(name, String(values.get("provider")), String(values.get("baseUrl")), String(values.get("model")), secretName, alias, agentName, { stream: values.get("stream") === "on", tools: values.get("tools") === "on" });
    setBusy(true); setError("");
    try {
      const plan = await api.planConfiguration(configuration);
      (form.elements.namedItem("secret") as HTMLInputElement).value = "";
      setPending({ configuration, plan, secretName, secret, agentName, alias, configured: false, tokenRecovery: false });
    } catch (reason) {
      const code = reason instanceof APIError ? reason.code : "request_failed";
      setError(code === "already_exists" ? "Ese nombre ya existe. Elige otro identificador." : `No se pudo completar el asistente (${code}). Puedes reintentarlo sin perder la API key ya guardada.`);
    } finally { setBusy(false); }
  }

  function complete(token: string, current: Exclude<Pending, null>) { setResult({token,alias:current.alias});setPending(null);onComplete(); }
  async function apply() { if(!pending)return; setBusy(true);setError("");let current=pending;try{if(!current.configured){await api.saveSecret(current.secretName,current.secret);try{await api.applyConfiguration(current.configuration,current.plan.planToken);}catch(reason){const refreshed=await api.planConfiguration(current.configuration);if(refreshed.changes.length){setPending({...current,plan:refreshed});throw reason;}}current={...current,configured:true};setPending(current);}try{const issued=await api.issueAgentToken(current.agentName);complete(issued.token,current);}catch(reason){const status=await api.agentTokenStatus(current.agentName);if(status.tokenStatus.state==="active"){setPending({...current,tokenRecovery:true});setError("La ruta quedó creada, pero el token ya fue emitido y su respuesta se perdió. Revócalo y emite uno nuevo para continuar.");return;}throw reason;}}catch(reason){const code=reason instanceof APIError?reason.code:"request_failed";setError(`No se pudo completar el asistente (${code}). Revisa el estado y vuelve a intentarlo; la API key guardada no se pierde.`);}finally{setBusy(false);} }
  async function replaceToken(){if(!pending)return;setBusy(true);setError("");try{await api.revokeAgentToken(pending.agentName);const issued=await api.issueAgentToken(pending.agentName);complete(issued.token,pending);}catch(reason){const code=reason instanceof APIError?reason.code:"request_failed";setError(`No se pudo reemplazar el token (${code}). Comprueba su estado antes de reintentar.`);}finally{setBusy(false);}}

  if (result) return <div className="dialog-backdrop"><section ref={dialog} tabIndex={-1} onKeyDown={dialogKeys} className="wizard result" role="dialog" aria-modal="true" aria-labelledby="token-title"><p className="step">RUTA CREADA</p><h2 id="token-title">Guarda este token ahora</h2><p>Por seguridad, ModelCairn no podrá volver a mostrarlo. Úsalo con el alias <strong>{result.alias}</strong>.</p><output>{result.token}</output><button onClick={() => navigator.clipboard.writeText(result.token)}>Copiar token</button><button className="secondary" onClick={onClose}>Terminar</button></section></div>;

  if (pending) return <div className="dialog-backdrop"><section ref={dialog} tabIndex={-1} onKeyDown={dialogKeys} className="wizard" role="dialog" aria-modal="true" aria-labelledby="review-title"><p className="step">REVISIÓN</p><h2 id="review-title">{pending.tokenRecovery?"Recupera el acceso":"Confirma los cambios"}</h2><p>{pending.tokenRecovery?"El token anterior no puede volver a mostrarse. La ruta y la API key ya están guardadas.":"ModelCairn validó el plan. Nada se modificará hasta que confirmes."}</p>{!pending.tokenRecovery&&<ul className="plan-list">{pending.plan.changes.map((change,index)=><li key={`${change.kind}-${change.name}-${index}`}><strong>{change.operation}</strong> {change.kind} <code>{change.name}</code></li>)}</ul>}{error&&<div className="form-error" role="alert">{error}</div>}<div className="wizard-actions"><button className="secondary" type="button" onClick={()=>setPending(null)} disabled={busy}>Volver</button><button onClick={pending.tokenRecovery?replaceToken:apply} disabled={busy}>{busy?"Aplicando…":pending.tokenRecovery?"Revocar y emitir token nuevo":"Confirmar y crear"}</button></div></section></div>;

  return <div className="dialog-backdrop"><section className="wizard" role="dialog" aria-modal="true" aria-labelledby="wizard-title"><div className="wizard-head"><div><p className="step">PRIMERA RUTA</p><h2 id="wizard-title">Conecta un proveedor</h2></div><button className="close" onClick={onClose} aria-label="Cerrar">×</button></div><p>Crearemos proveedor, conexión, modelo, destino, estrategia, ruta y agente en una operación validada.</p><form onSubmit={submit}><div className="field-grid"><label>Nombre del proveedor<input name="provider" required maxLength={120} placeholder="OpenRouter" /></label><label>Identificador<input name="name" required pattern="[a-z][a-z0-9-]{0,62}" placeholder="openrouter" aria-describedby="name-help"/><small id="name-help">Minúsculas, números y guiones.</small></label></div><label>URL base de la API<input name="baseUrl" type="url" required defaultValue="https://openrouter.ai/api/v1" /></label><label>Modelo del proveedor<input name="model" required maxLength={255} placeholder="openai/gpt-oss-20b:free" /></label><fieldset><legend>Capacidades verificadas del modelo</legend><label><input name="stream" type="checkbox"/> Streaming SSE</label><label><input name="tools" type="checkbox"/> Tool calls</label><small>Activa solo capacidades que el proveedor documenta para este modelo. Texto es la única capacidad predeterminada.</small></fieldset><label>API key<input name="secret" type="password" required minLength={8} maxLength={16384} autoComplete="off"/><small>Se cifra en el nodo principal y nunca vuelve a mostrarse.</small></label><div className="field-grid"><label>Alias para tus agentes<input name="alias" required pattern="[A-Za-z0-9._:/-]{1,128}" defaultValue="assistant" /></label><label>Nombre del agente<input name="agent" required pattern="[a-z][a-z0-9-]{0,62}" defaultValue="my-agent" /></label></div>{error && <div className="form-error" role="alert">{error}</div>}<div className="wizard-actions"><button className="secondary" type="button" onClick={onClose}>Cancelar</button><button type="submit" disabled={busy}>{busy ? "Creando ruta…" : "Revisar y crear"}</button></div></form></section></div>;
}
