import { FormEvent, useEffect, useState } from "react";
import { api, APIError, SessionContext } from "./api/client";

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
  return <div className="app-shell"><header><Brand/><div className="header-actions"><span className="signal"><span className={online ? "dot online" : "dot"}/>{online ? "En línea" : "Sin conexión"}</span><button className="text-button" onClick={onLogout}>Cerrar sesión</button></div></header><aside aria-label="Navegación principal"><nav><a className="active" href="#overview">Resumen</a><a href="#onboarding">Crear primera ruta</a><a href="#resources">Recursos</a><a href="#activity">Actividad</a><a href="#settings">Configuración</a></nav><a className="repo-link" href="https://github.com/sxamx/modelcairn" rel="noreferrer">ModelCairn · Código abierto</a></aside><main className="content" id="overview"><p className="eyebrow">RESUMEN</p><h1>Hola, {session.admin.username}</h1><p className="lede">Tu consola ya está conectada. El siguiente bloque añadirá el estado real de rutas, destinos y actividad.</p><section className="empty-state"><span className="brand-mark large" aria-hidden="true"><i/><i/><i/></span><div><h2>Construyamos tu primera ruta</h2><p>El asistente te guiará desde el proveedor hasta un token listo para tu agente.</p></div><button disabled>Comenzar configuración</button></section></main></div>;
}
