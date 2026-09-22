import { FormEvent, useCallback, useEffect, useState } from "react";
import { api, APIError, Overview, Readiness, SessionContext } from "./api/client";
import Onboarding from "./Onboarding";
import Catalog from "./Catalog";
import Secrets from "./Secrets";
import AgentTokens from "./AgentTokens";
import Activity from "./Activity";
import Settings from "./Settings";
import Data from "./Data";

type SessionState = { kind: "loading" } | { kind: "anonymous" } | { kind: "authenticated"; value: SessionContext };
type ConsolePage = "overview" | "providers" | "models" | "routes" | "secrets" | "tokens" | "activity" | "data" | "settings";
const pages: { id: ConsolePage; label: string; icon: string }[] = [
  { id: "overview", label: "Inicio", icon: "home" },
  { id: "providers", label: "Proveedores", icon: "providers" },
  { id: "models", label: "Modelos", icon: "models" },
  { id: "routes", label: "Rutas", icon: "routes" },
  { id: "tokens", label: "Acceso API", icon: "access" },
  { id: "activity", label: "Actividad", icon: "activity" },
  { id: "data", label: "Datos", icon: "data" },
  { id: "settings", label: "Configuración", icon: "settings" },
];
function pageFromHash(): ConsolePage {
  const name = window.location.hash.slice(2).split("?")[0].split("/")[0];
  if (name === "secrets") return "secrets";
  return pages.find((item) => item.id === name)?.id ?? "overview";
}
function Icon({ name }: { name: string }) {
  const paths: Record<string, React.ReactNode> = {
    home: <><path d="m3 11 9-8 9 8"/><path d="M5 10v10h14V10"/></>,
    providers: <><rect x="3" y="4" width="18" height="6" rx="1.5"/><rect x="3" y="14" width="18" height="6" rx="1.5"/><path d="M7 7h.01M7 17h.01"/></>,
    models: <><path d="m12 3 9 5-9 5-9-5 9-5z"/><path d="m3 13 9 5 9-5"/></>,
    routes: <><circle cx="6" cy="18" r="2.5"/><circle cx="18" cy="6" r="2.5"/><path d="M8.5 18H15a3 3 0 0 0 0-6H9a3 3 0 0 1 0-6h6.5"/></>,
    key: <><circle cx="8" cy="15" r="4"/><path d="m11 12 9-9M16 7l3 3"/></>,
    access: <><rect x="3" y="4" width="18" height="16" rx="2"/><path d="M7 9h10M7 14h6"/></>,
    activity: <path d="M3 12h4l3-8 4 16 3-8h4"/>,
    data: <path d="M4 20V10M10 20V4M16 20v-7M22 20H2"/>,
    settings: <><circle cx="12" cy="12" r="3"/><path d="M12 2v3M12 19v3M2 12h3M19 12h3M5 5l2 2M17 17l2 2M5 19l2-2M17 7l2-2"/></>,
    panel: <><rect x="3" y="3" width="18" height="18" rx="2"/><path d="M9 3v18"/></>,
    sun: <><circle cx="12" cy="12" r="4"/><path d="M12 2v2M12 20v2M2 12h2M20 12h2M5 5l1.5 1.5M17.5 17.5 19 19M5 19l1.5-1.5M17.5 6.5 19 5"/></>,
    moon: <path d="M21 13A9 9 0 0 1 11 3a9 9 0 1 0 10 10Z"/>,
  };
  return <svg className="console-icon" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}
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
    window.addEventListener("online", update); window.addEventListener("offline", update);
    return () => { window.removeEventListener("online", update); window.removeEventListener("offline", update); };
  }, []);
  if (session.kind === "loading") return <Loading />;
  if (session.kind === "anonymous") return <Login onAuthenticated={(value) => setSession({ kind: "authenticated", value })} online={online} />;
  return <Console session={session.value} online={online} onLogout={() => { void api.logout().then(() => setSession({ kind: "anonymous" })); }} />;
}

function Brand({ href = "/" }: { href?: string }) { return <a className="brand" href={href} aria-label="ModelCairn, inicio"><span className="brand-mark" aria-hidden="true"><i/><i/><i/></span><span>ModelCairn</span></a>; }
function Loading() { return <main className="center"><Brand /><div className="loading" role="status"><span className="spinner" />Comprobando sesión…</div></main>; }

function Login({ onAuthenticated, online }: { onAuthenticated: (session: SessionContext) => void; online: boolean }) {
  const [busy, setBusy] = useState(false); const [message, setMessage] = useState("");
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault(); setBusy(true); setMessage("");
    const element = event.currentTarget; const form = new FormData(element);
    try { const value = await api.login(String(form.get("username") ?? ""), String(form.get("password") ?? "")); element.reset(); onAuthenticated(value); }
    catch (error) { const code = error instanceof APIError ? error.code : "request_failed"; setMessage(errorText[code] ?? "No pudimos iniciar sesión. Revisa los datos e inténtalo nuevamente."); }
    finally { setBusy(false); }
  }
  return <main className="login-layout"><section className="login-intro"><Brand /><p className="eyebrow">Tu infraestructura, tus rutas</p><h1>Un punto claro entre tus agentes y tus proveedores.</h1><p className="lede">Configura, observa y recupera cada ruta desde una consola privada y ligera.</p><div className="signal"><span className={online ? "dot online" : "dot"}/>{online ? "Dispositivo conectado" : "Conectividad no confirmada"}</div></section><section className="login-panel" aria-labelledby="login-title"><div className="panel-card"><p className="step">CONSOLA ADMINISTRATIVA</p><h2 id="login-title">Bienvenido de vuelta</h2><p>Inicia sesión con el administrador creado durante la configuración inicial.</p><form onSubmit={submit}><label htmlFor="username">Usuario</label><input id="username" name="username" autoComplete="username" required maxLength={128} disabled={busy}/><label htmlFor="password">Contraseña</label><input id="password" name="password" type="password" autoComplete="current-password" required disabled={busy}/>{message && <div className="form-error" role="alert">{message}</div>}<button type="submit" disabled={busy}>{busy ? "Entrando…" : "Entrar a ModelCairn"}</button></form><p className="private-note">La sesión y tus datos permanecen en esta instalación.</p></div></section></main>;
}

function Console({ session, online, onLogout }: { session: SessionContext; online: boolean; onLogout: () => void }) {
  const [overview, setOverview] = useState<Overview | null>(null);
  const [readiness, setReadiness] = useState<Readiness | null>(null);
  const [unavailable, setUnavailable] = useState(false);
  const [onboarding, setOnboarding] = useState(false);
  const [page, setPage] = useState<ConsolePage>(pageFromHash);
  const [collapsed, setCollapsed] = useState(() => localStorage.getItem("modelcairn-sidebar-collapsed") === "true");
  const [theme, setTheme] = useState<"light" | "dark">(() => localStorage.getItem("modelcairn-theme") === "dark" ? "dark" : "light");
  const [profileOpen, setProfileOpen] = useState(false);
  useEffect(() => {
    const sync = () => { setPage(pageFromHash()); setProfileOpen(false); };
    window.addEventListener("hashchange", sync);
    return () => window.removeEventListener("hashchange", sync);
  }, []);
  useEffect(() => { localStorage.setItem("modelcairn-theme", theme); }, [theme]);
  useEffect(() => { localStorage.setItem("modelcairn-sidebar-collapsed", String(collapsed)); }, [collapsed]);
  function navigate(next: ConsolePage) {
    setProfileOpen(false);
    if (window.location.hash === `#/${next}`) return;
    window.location.hash = `/${next}`;
    setPage(next);
  }
  const load = useCallback(() => {
    setUnavailable(false);
    api.overview().then(setOverview).catch(() => setUnavailable(true));
    api.readiness().then(setReadiness).catch(() => setReadiness(null));
  }, []);
  useEffect(load, [load]);
  const routes = overview?.resourceCounts.Route ?? 0;
  return <div className={`app-shell console-redesign ${collapsed ? "is-collapsed" : ""}`} data-theme={theme}>
    <aside aria-label="Navegación principal"><Brand href="#/overview"/><nav>{pages.map((item) => <button key={item.id} className={page === item.id ? "active" : ""} onClick={() => navigate(item.id)} aria-current={page === item.id ? "page" : undefined} title={collapsed ? item.label : undefined}><Icon name={item.icon}/><span className="nav-label">{item.label}</span></button>)}</nav><a className="repo-link" href="https://github.com/sxamx/modelcairn" rel="noreferrer"><span className="nav-label">ModelCairn · Código abierto</span></a></aside>
    <div className="console-wrap"><header><button className="console-icon-button collapse-button" type="button" aria-label={collapsed ? "Expandir menú" : "Contraer menú"} onClick={() => setCollapsed((value) => !value)}><Icon name="panel"/></button><strong className="console-crumb">{page === "secrets" ? "Claves guardadas" : pages.find((item) => item.id === page)?.label}</strong><div className="header-actions"><span className="signal"><span className={online ? "dot online" : "dot"}/>{online ? "En línea" : "Sin conexión"}</span><button className="console-icon-button" type="button" aria-label={theme === "light" ? "Activar modo oscuro" : "Activar modo claro"} onClick={() => setTheme(theme === "light" ? "dark" : "light")}><Icon name={theme === "light" ? "moon" : "sun"}/></button><button className="console-avatar" type="button" aria-label="Menú de usuario" aria-expanded={profileOpen} onClick={() => setProfileOpen((value) => !value)}>{session.admin.username.slice(0, 1).toLocaleUpperCase()}</button></div>{profileOpen && <div className="console-profile-menu"><div><strong>{session.admin.username}</strong><small>Administrador</small></div><button onClick={() => navigate("settings")}>Configuración</button><button onClick={onLogout}>Cerrar sesión</button></div>}</header>
    <main className="content" id={page}>{page === "providers" ? <Catalog key="providers" area="providers" onOpenWizard={() => setOnboarding(true)} onOpenSecrets={() => { window.location.hash = `#/secrets?from=${encodeURIComponent(window.location.hash)}`; }}/> : page === "models" ? <Catalog key="models" area="models" onOpenWizard={() => setOnboarding(true)} onOpenSecrets={() => { window.location.hash = `#/secrets?from=${encodeURIComponent(window.location.hash)}`; }}/> : page === "routes" ? <Catalog key="routes" area="routes" onOpenWizard={() => setOnboarding(true)} onOpenSecrets={() => { window.location.hash = `#/secrets?from=${encodeURIComponent(window.location.hash)}`; }}/> : page === "secrets" ? <Secrets/> : page === "tokens" ? <AgentTokens/> : page === "activity" ? <Activity/> : page === "data" ? <Data overview={overview} unavailable={unavailable}/> : page === "settings" ? <Settings/> : <><h1>Hola, {session.admin.username}</h1><p className="lede">Una vista privada del estado y la actividad de esta instalación.</p>{unavailable && <div className="form-error" role="status">El resumen no está disponible. Tus rutas siguen funcionando de forma independiente.</div>}
      <section className="metric-grid" aria-label="Estado de la instalación"><Metric label="Gateway" value={readiness ? (readiness.status === "ready" ? "Listo" : "Requiere atención") : "Sin verificar"} tone={readiness?.status}/><Metric label="Rutas" value={overview ? routes : "—"}/><Metric label="Opciones de ruta" value={overview ? overview.resourceCounts.Destination ?? 0 : "—"}/><Metric label="Solicitudes · 24 h" value={overview ? overview.requests24h.total : "—"}/><Metric label="Opciones en pausa" value={overview ? overview.activeCooldowns : "—"}/></section>
      {readiness?.status === "not_ready" && <ReadinessNotice readiness={readiness}/>} 
      {overview && overview.recentRequests.length > 0 ? <Recent overview={overview}/> : <section className="empty-state"><span className="brand-mark large" aria-hidden="true"><i/><i/><i/></span><div><h2>{routes ? "Aún no hay solicitudes" : "Configura tu primer proveedor"}</h2><p>{routes ? "La actividad aparecerá aquí cuando un agente use el gateway." : "Puedes añadir recursos individualmente o usar el asistente para preparar una ruta completa."}</p></div>{routes === 0 && <button onClick={() => setOnboarding(true)}>Abrir asistente</button>}</section>}
    </>}</main></div>{onboarding && <Onboarding onClose={() => setOnboarding(false)} onComplete={load}/>}</div>;
}

const outcomeLabels: Record<string, string> = { success: "Correcta", error: "Error", partial: "Parcial", cancelled: "Cancelada", indeterminate: "Indeterminada" };
function Recent({ overview }: { overview: Overview }) { return <section className="recent"><div><p className="step">ACTIVIDAD RECIENTE</p><h2>Últimas solicitudes</h2></div><ul>{overview.recentRequests.map((item) => <li key={item.id}><span><strong>{item.requestedAlias}</strong><small>{new Date(item.startedAt).toLocaleString()}</small></span><span className={`outcome ${item.outcome ?? "pending"}`}>{item.outcome ? (outcomeLabels[item.outcome] ?? "Otro resultado") : "En curso"}</span><span>{item.durationMs == null ? "—" : `${item.durationMs} ms`}</span></li>)}</ul></section>; }
function Metric({ label, value, tone }: { label: string; value: string | number; tone?: Readiness["status"] }) { return <article className={`metric ${tone ?? ""}`}><span>{label}</span><strong>{value}</strong></article>; }
function ReadinessNotice({ readiness }: { readiness: Readiness }) {
  const affected = Object.entries(readiness.components).filter(([, state]) => !state.ready);
  return <section className="readiness-notice" role="status"><strong>La instalación necesita atención</strong><ul>{affected.map(([name, state]) => <li key={name}>{name}: {state.reason || "no disponible"}</li>)}</ul></section>;
}
