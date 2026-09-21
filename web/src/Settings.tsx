import { FormEvent, ReactNode, useEffect, useState } from "react";
import { api, APIError, SettingsDocument, SettingsPlan, SettingsSpec, SettingsState } from "./api/client";

const number=(data:FormData,name:string)=>Number(data.get(name));
export default function Settings(){
  const [state,setState]=useState<SettingsState|null>(null);const [plan,setPlan]=useState<SettingsPlan|null>(null);const [draft,setDraft]=useState<SettingsDocument|null>(null);const [message,setMessage]=useState("");const [busy,setBusy]=useState(false);
  const load=()=>api.getSettings().then(setState).catch(()=>setMessage("No pudimos cargar la configuración."));useEffect(()=>{void load()},[]);
  async function review(event:FormEvent<HTMLFormElement>){event.preventDefault();if(!state)return;const data=new FormData(event.currentTarget);const spec:SettingsSpec={publicOrigin:String(data.get("publicOrigin")),listen:String(data.get("listen")),transport:String(data.get("transport")) as SettingsSpec["transport"],trustedProxyCidrs:String(data.get("trustedProxyCidrs")).split(/\r?\n|,/).map(x=>x.trim()).filter(Boolean),tlsCertificatePath:String(data.get("tlsCertificatePath")),tlsPrivateKeyPath:String(data.get("tlsPrivateKeyPath")),idleSeconds:number(data,"idleSeconds"),absoluteSeconds:number(data,"absoluteSeconds"),globalAttemptsPerMinute:number(data,"globalAttemptsPerMinute"),globalBurst:number(data,"globalBurst"),clientAttemptsPerMinute:number(data,"clientAttemptsPerMinute"),clientBurst:number(data,"clientBurst"),maxClientEntries:number(data,"maxClientEntries"),clientIdleSeconds:number(data,"clientIdleSeconds"),failedLoginRetentionSeconds:number(data,"failedLoginRetentionSeconds"),argonMemoryKiB:number(data,"argonMemoryKiB"),argonIterations:number(data,"argonIterations")};const document:SettingsDocument={apiVersion:"modelcairn.io/v1alpha1",kind:"AdminSettings",resourceVersion:state.desired.resourceVersion,spec};setBusy(true);setMessage("");try{setPlan(await api.planSettings(document));setDraft(document);}catch(reason){setMessage(reason instanceof APIError?`La configuración no es válida (${reason.code}). Revisa las relaciones entre límites y transporte.`:"No pudimos revisar la configuración.");}finally{setBusy(false)}}
  async function apply(){if(!plan||!draft)return;setBusy(true);try{const result=await api.applySettings(draft,plan.planToken);setState(result.settings as SettingsState);setPlan(null);setDraft(null);setMessage("Configuración guardada correctamente.");}catch(reason){setMessage(reason instanceof APIError?`No se pudo aplicar (${reason.code}). Vuelve a revisar los cambios.`:"No se pudo aplicar la configuración.");setPlan(null);}finally{setBusy(false)}}
  if(!state)return <div className="loading" role="status"><span className="spinner"/>{message||"Cargando configuración…"}</div>;
  const s=state.desired.spec;
  return <section aria-labelledby="settings-title">
    <h1 id="settings-title">Configuración</h1>
    <p className="lede">Ajustes de esta instalación. Revisamos los cambios antes de guardarlos; los que afectan al servicio se activan al reiniciar.</p>
    {state.restartRequired && <div className="restart-banner" role="status">Hay cambios guardados pendientes de reinicio.</div>}
    {message && <div className="notice" role="status">{message}</div>}
    {plan && draft ? <section className="settings-review">
      <p className="step">REVISIÓN</p>
      <h2>{plan.changedFields.length ? plan.changedFields.length + " cambios listos para aplicar" : "No hay diferencias"}</h2>
      <ul>{plan.changedFields.map(field => <li key={field}>{field}</li>)}</ul>
      <p>Guardar no reiniciará el servicio ni interrumpirá esta sesión.</p>
      <div><button className="secondary" onClick={() => { setPlan(null); setDraft(null); }}>Volver a editar</button><button onClick={apply} disabled={busy}>{busy ? "Aplicando…" : "Aplicar configuración"}</button></div>
    </section> : <form className="settings-form" key={state.desired.resourceVersion} onSubmit={review}>
      <div className="settings-primary">
        <section className="settings-card" aria-labelledby="settings-access-title">
          <h2 id="settings-access-title">Acceso a la consola</h2>
          <p>La dirección que usas para entrar y el tiempo que permanece abierta tu sesión.</p>
          <div className="settings-grid"><Field name="publicOrigin" label="URL de acceso" value={s.publicOrigin}/><NumberField name="idleSeconds" label="Cerrar sesión tras inactividad (segundos)" value={s.idleSeconds} min={300} max={86400}/></div>
        </section>
        <section className="settings-card" aria-labelledby="settings-history-title">
          <h2 id="settings-history-title">Historial de acceso fallido</h2>
          <p>Cuánto tiempo conservar los intentos de inicio de sesión rechazados.</p>
          <label>Historial de login fallido (segundos)<input name="failedLoginRetentionSeconds" type="number" defaultValue={s.failedLoginRetentionSeconds} min={0} max={3155760000}/><small>24 horas = 86400. Usa 0 para conservarlo indefinidamente; puede crecer con el tiempo.</small></label>
        </section>
      </div>
      <details className="settings-advanced"><summary>Red y seguridad avanzadas</summary>
        <p>Cambia estos valores solo si conoces tu instalación. Un ajuste incorrecto de red podría impedir el acceso a la consola.</p>
        <SettingsSection title="Red y HTTPS">
          <Field name="listen" label="Dirección de escucha" value={s.listen}/>
          <label>Transporte<select name="transport" defaultValue={s.transport}><option value="loopback-http">HTTP local</option><option value="direct-tls">TLS directo</option><option value="proxy-tls">TLS mediante proxy</option></select></label>
          <label className="wide">CIDR de proxies confiables<textarea name="trustedProxyCidrs" defaultValue={s.trustedProxyCidrs.join("\n")} rows={2}/><small>Uno por línea; solo se usan en modo proxy-tls.</small></label>
          <Field name="tlsCertificatePath" label="Ruta del certificado TLS" value={s.tlsCertificatePath}/>
          <Field name="tlsPrivateKeyPath" label="Ruta de la clave TLS" value={s.tlsPrivateKeyPath}/>
        </SettingsSection>
        <SettingsSection title="Límites de acceso">
          <NumberField name="absoluteSeconds" label="Duración máxima de sesión (segundos)" value={s.absoluteSeconds} min={300} max={604800}/>
          <NumberField name="globalAttemptsPerMinute" label="Intentos globales por minuto" value={s.globalAttemptsPerMinute} min={1} max={120}/>
          <NumberField name="globalBurst" label="Ráfaga global" value={s.globalBurst} min={1} max={20}/>
          <NumberField name="clientAttemptsPerMinute" label="Intentos por cliente/minuto" value={s.clientAttemptsPerMinute} min={1} max={30}/>
          <NumberField name="clientBurst" label="Ráfaga por cliente" value={s.clientBurst} min={1} max={10}/>
          <NumberField name="maxClientEntries" label="Clientes rastreados" value={s.maxClientEntries} min={64} max={4096}/>
          <NumberField name="clientIdleSeconds" label="Olvidar cliente inactivo (segundos)" value={s.clientIdleSeconds} min={60} max={3600}/>
        </SettingsSection>
        <SettingsSection title="Criptografía">
          <NumberField name="argonMemoryKiB" label="Memoria Argon2id (KiB)" value={s.argonMemoryKiB} min={19456} max={65536}/>
          <NumberField name="argonIterations" label="Iteraciones Argon2id" value={s.argonIterations} min={2} max={6}/>
        </SettingsSection>
      </details>
      <button className="review-button" disabled={busy}>{busy ? "Validando…" : "Revisar cambios"}</button>
    </form>}
  </section>;
}
function SettingsSection({title,children}:{title:string;children:ReactNode}){return <fieldset><legend>{title}</legend><div className="settings-grid">{children}</div></fieldset>}
function Field({name,label,value}:{name:string;label:string;value:string}){return <label>{label}<input name={name} defaultValue={value} required={name==="publicOrigin"||name==="listen"}/></label>}
function NumberField({name,label,value,min,max}:{name:string;label:string;value:number;min:number;max:number}){return <label>{label}<input name={name} type="number" defaultValue={value} min={min} max={max} required/></label>}
