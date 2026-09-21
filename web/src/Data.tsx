import type { Overview } from "./api/client";

export default function Data({ overview, unavailable }: { overview: Overview | null; unavailable: boolean }) {
  const total = overview?.requests24h.total;
  const success = overview?.requests24h.success;
  const successRate = total && success !== undefined ? `${(success / total * 100).toLocaleString("es", { maximumFractionDigits: 1 })} %` : "—";
  return <section aria-labelledby="data-title" className="data-page">
    <h1 id="data-title">Datos</h1>
    <p className="lede">Métricas operativas de esta instalación. No se guardan prompts ni respuestas.</p>
    {unavailable && <div className="form-error" role="status">No pudimos cargar las métricas. Inténtalo nuevamente más tarde.</div>}
    <div className="metric-grid" aria-label="Últimas 24 horas">
      <article className="metric"><span>Solicitudes · 24 h</span><strong>{total ?? "—"}</strong></article>
      <article className="metric"><span>Tasa de éxito · 24 h</span><strong>{successRate}</strong></article>
      <article className="metric"><span>Errores · 24 h</span><strong>{overview?.requests24h.error ?? "—"}</strong></article>
      <article className="metric"><span>Cooldowns activos</span><strong>{overview?.activeCooldowns ?? "—"}</strong></article>
    </div>
    <div className="data-grid">
      <article className="data-panel"><h2>Latencia y tokens por segundo</h2><p>Sin datos agregados todavía. La vista de Actividad muestra el tiempo y los tokens de cada solicitud registrada.</p></article>
      <article className="data-panel"><h2>Costos estimados</h2><p>Sin datos. Primero tendremos que configurar precios por modelo y acordar cómo se calculan; no equivaldrán a facturas del proveedor.</p></article>
    </div>
  </section>;
}
