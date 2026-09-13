import { FormEvent, useEffect, useState } from "react";
import { api, APIError, Overview, SessionContext } from "./api/client";

type SessionState = { kind: "loading" } | { kind: "anonymous" } | { kind: "authenticated"; value: SessionContext };

const errorText: Record<string, string> = {
  authentication_required: "La sesión no es válida o ya expiró.",
  invalid_credentials: "El usuario o la contraseña no son correctos.",
  rate_limited: "Demasiados intentos. Espera un momento antes de volver a probar.",
  request_failed: "No pudimos completar la solicitud.",
};

export default function App() {
  const [session, setSession] = useState<SessionState>({ kind: "loading" });
  const [online, setOnline] = useState(navigator.onLine);

  useEffect(() => {
    api.session().then((value) => setSession({ kind: "authenticated", value })).catch(() => setSession({ kind: "anonymous" }));
    const update = () => setOnline(navigator.onLine);
    window.addEventListener("online", update);
    window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);

  if (session.kind === "loading") return <Loading />;
  if (session.kind === "anonymous") return <Login onAuthenticated={(value) => setSession({ kind: "authenticated", value })} online={online} />;
  return <Console session={session.value} online={online} onLogout={() => api.logout().finally(() => setSession({ kind: "anonymous" }))} />;
}

function Brand() {
  return <a className="brand" href="/" aria-label="ModelCairn, inicio"><span className="brand-mark" aria-hidden="true"><i/><i/><i/></span><span>ModelCairn</span></a>;
}

function Loading() {
  return <main className="center"><Brand /><div className="loading" role="status"><span className="spinner" />Comprobando sesión…</div></main>;
}

function Login({ onAuthenticated, online }: { onAuthenticated: (session: SessionContext) => void; online: boolean }) {
  const [busy, setBusy] = useState(false);
  const [message, setMessage] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    const element = event.currentTarget;
    const form = new FormData(element);
    try {
      const value = await api.login(String(form.get("username") ?? ""), String(form.get("password") ?? ""));
      element.reset(); onAuthenticated(value);
    } catch (error) {
      const code = error instanceof APIError ? error.code : "request_failed";
      setMessage(errorText[code] ?? "No pudimos iniciar sesión. Revisa los datos e inténtalo nuevamente.");
    } finally { setBusy(false); }
  }
  return <main className="login-layout">
    <section className="login-intro"><Brand /><p className="eyebrow">Tu infraestructura, tus rutas</p><h1>Un punto claro entre tus agentes y tus proveedores.</h1><p className="lede">Configura, observa y recupera cada ruta desde una consola privada y ligera.</p><div className="signal"><span className={online ? "dot online" : "dot"}/>{online ? "Dispositivo conectado" : "Sin conexión"}</div></section>
    <section className="login-panel" aria-labelledby="login-title"><div className="panel-card"><p className="step">CONSOLA ADMINISTRATIVA</p><h2 id="login-title">Bienvenido de vuelta</h2><p>Inicia sesión con el administrador creado durante la configuración inicial.</p><form onSubmit={submit}><label htmlFor="username">Usuario</label><input id="username" name="username" autoComplete="username" required maxLength={128} disabled={busy}/><label htmlFor="password">Contraseña</label><input id="password" name="password" type="password" autoComplete="current-password" required disabled={busy}/>{message && <div className="form-error" role="alert">{message}</div>}<button type="submit" disabled={busy || !online}>{busy ? "Entrando…" : "Entrar a ModelCairn"}</button></form><p className="private-note">La sesión y tus datos permanecen en esta instalación.</p></div></section>
  </main>;
}

function Console({ session, online, onLogout }: { session: SessionContext; online: boolean; onLogout: () => void }) {
  const [overview, setOverview] = useState<Overview | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  useEffect(() => { api.overview().then(setOverview).catch(() => setUnavailable(true)); }, []);
  const routes = overview?.resourceCounts.Route ?? 0;
  return <div className="app-shell"><header><Brand/><div className="header-actions"><span className="signal"><span className={online ? "dot online" : "dot"}/>{online ? "En línea" : "Sin conexión"}</span><button className="text-button" onClick={onLogout}>Cerrar sesión</button></div></header><aside aria-label="Navegación principal"><nav><a className="active" href="#overview">Resumen</a><a href="#onboarding">Crear primera ruta</a><a href="#resources">Recursos</a><a href="#activity">Actividad</a><a href="#settings">Configuración</a></nav><a className="repo-link" href="https://github.com/sxamx/modelcairn" rel="noreferrer">ModelCairn · Código abierto</a></aside><main className="content" id="overview"><p className="eyebrow">RESUMEN</p><h1>Hola, {session.admin.username}</h1><p className="lede">Una vista privada del estado y la actividad de esta instalación.</p>{unavailable && <div className="form-error" role="status">El resumen no está disponible. Tus rutas siguen funcionando de forma independiente.</div>}<section className="metric-grid" aria-label="Estado de la instalación"><Metric label="Rutas" value={overview ? routes : "—"}/><Metric label="Destinos" value={overview ? overview.resourceCounts.Destination ?? 0 : "—"}/><Metric label="Solicitudes · 24 h" value={overview ? overview.requests24h.total : "—"}/><Metric label="Cooldowns activos" value={overview ? overview.activeCooldowns : "—"}/></section>{overview && overview.recentRequests.length > 0 ? <section className="recent"><div><p className="step">ACTIVIDAD RECIENTE</p><h2>Últimas solicitudes</h2></div><ul>{overview.recentRequests.map((item) => <li key={item.id}><span><strong>{item.requestedAlias}</strong><small>{new Date(item.startedAt).toLocaleString()}</small></span><span className={`outcome ${item.outcome ?? "pending"}`}>{item.outcome ?? "en curso"}</span><span>{item.durationMs == null ? "—" : `${item.durationMs} ms`}</span></li>)}</ul></section> : <section className="empty-state"><span className="brand-mark large" aria-hidden="true"><i/><i/><i/></span><div><h2>{routes ? "Aún no hay solicitudes" : "Construyamos tu primera ruta"}</h2><p>{routes ? "La actividad aparecerá aquí cuando un agente use el gateway." : "El asistente te guiará desde el proveedor hasta un token listo para tu agente."}</p></div><button disabled>Comenzar configuración</button></section>}</main></div>;
}

function Metric({ label, value }: { label: string; value: string | number }) { return <article className="metric"><span>{label}</span><strong>{value}</strong></article>; }
