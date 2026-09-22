import { useEffect, useMemo, useState } from "react";
import { api, OperationalMetrics, Overview, Resource } from "./api/client";

type Price = { currency: "USD"; inputPerMillion: number; outputPerMillion: number };
type Cost = { value: number | null; pricedRequests: number; reportedRequests: number };
const number = new Intl.NumberFormat("es", { maximumFractionDigits: 0 });
const money = new Intl.NumberFormat("es", { style: "currency", currency: "USD", minimumFractionDigits: 2, maximumFractionDigits: 4 });

function priceOf(model: Resource | undefined): Price | null {
  if (!model || !("spec" in model) || !model.spec || typeof model.spec !== "object") return null;
  const value = (model.spec as Record<string, unknown>).pricing;
  if (!value || typeof value !== "object") return null;
  const price = value as Record<string, unknown>;
  return price.currency === "USD" && typeof price.inputPerMillion === "number" && typeof price.outputPerMillion === "number" ? price as Price : null;
}
function modelName(model: Resource): string { return String((model.metadata as { name?: string }).name ?? ""); }
function costFor(metrics: OperationalMetrics | null, models: Resource[]): Cost {
  if (!metrics) return { value: null, pricedRequests: 0, reportedRequests: 0 };
  const byName = new Map(models.map(model => [modelName(model), model]));
  let value = 0, pricedRequests = 0;
  for (const row of metrics.models) {
    const price = priceOf(byName.get(row.name));
    if (!price) continue;
    value += (row.inputTokens * price.inputPerMillion + row.outputTokens * price.outputPerMillion) / 1_000_000;
    pricedRequests += row.requests;
  }
  return { value: pricedRequests ? value : null, pricedRequests, reportedRequests: metrics.usageKnownRequests };
}
function days(metrics: OperationalMetrics, count: number) {
  const byDate = new Map(metrics.daily.map(day => [day.date, day]));
  const end = new Date(metrics.generatedAt);
  end.setUTCHours(0, 0, 0, 0);
  return Array.from({ length: count }, (_, index) => {
    const date = new Date(end.getTime() - (count - index - 1) * 86400000).toISOString().slice(0, 10);
    const value = byDate.get(date);
    return { date, requests: value?.requests ?? 0, tokens: (value?.inputTokens ?? 0) + (value?.outputTokens ?? 0) };
  });
}
function demonstration(): OperationalMetrics {
  const generatedAt = new Date().toISOString();
  const sample = { requests: 0, inputTokens: 0, outputTokens: 0, usageKnownRequests: 0, daily: [] as OperationalMetrics["daily"], models: [] as OperationalMetrics["models"], generatedAt };
  const end = new Date(generatedAt); end.setUTCHours(0, 0, 0, 0);
  for (let i = 89; i >= 0; i--) {
    const age = 89 - i;
    const requests = age < 48 ? 0 : (i % 7 === 0 ? 0 : 2 + (i * 7) % 13);
    const inputTokens = requests * (720 + (i % 5) * 110);
    const outputTokens = requests * (190 + (i % 4) * 45);
    sample.requests += requests; sample.inputTokens += inputTokens; sample.outputTokens += outputTokens; sample.usageKnownRequests += requests;
    sample.daily.push({ date: new Date(end.getTime() - i * 86400000).toISOString().slice(0, 10), requests, success: Math.max(0, requests - (i % 11 === 0 ? 1 : 0)), inputTokens, outputTokens });
  }
  sample.models.push({ name: "modelo-de-ejemplo", requests: sample.requests, inputTokens: sample.inputTokens, outputTokens: sample.outputTokens, avgLatencyMillis: 980, outputTokensPerSecond: 28.4 });
  return sample;
}

export default function Data({ overview, unavailable }: { overview: Overview | null; unavailable: boolean }) {
  const [metrics, setMetrics] = useState<OperationalMetrics | null>(null);
  const [models, setModels] = useState<Resource[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState(false);
  const [demo, setDemo] = useState(false);
  const example = useMemo(demonstration, []);
  useEffect(() => {
    let active = true;
    api.metrics().then(value => { if (active) setMetrics(value); }).catch(() => { if (active) setError(true); }).finally(() => { if (active) setLoading(false); });
    async function loadPrices() {
      const found: Resource[] = []; let cursor: string | undefined;
      for (let page = 0; page < 50; page++) {
        const result = await api.listResources("models", cursor);
        found.push(...result.items); cursor = result.nextCursor || undefined;
        if (!cursor) { if (active) setModels(found); return; }
      }
    }
    void loadPrices().catch(() => { /* Usage remains visible without prices. */ });
    return () => { active = false; };
  }, []);
  const view = demo ? example : metrics;
  const visibleModels = demo ? [{ kind: "Model", metadata: { name: "modelo-de-ejemplo", displayName: "Modelo de ejemplo" }, spec: { pricing: { currency: "USD", inputPerMillion: 0.15, outputPerMillion: 0.6 } } } as Resource] : models;
  const prices = costFor(view, visibleModels);
  const totalTokens = view ? view.inputTokens + view.outputTokens : null;
  const recent = view ? days(view, 30) : [];
  const calendar = view ? days(view, 90) : [];
  const max = Math.max(1, ...recent.map(day => day.tokens));
  const successRate = demo ? "96,8 %" : overview?.requests24h.total ? `${(overview.requests24h.success / overview.requests24h.total * 100).toLocaleString("es", { maximumFractionDigits: 1 })} %` : "—";
  return <section aria-labelledby="data-title" className="data-page data-insights">
    <div className="data-title-row"><div><p className="eyebrow">OBSERVABILIDAD LOCAL</p><h1 id="data-title">Datos</h1><p className="lede">Solicitudes, tokens y costo estimado del historial que conserva esta instalación.</p></div><button type="button" className="secondary data-demo-toggle" aria-pressed={demo} onClick={() => setDemo(!demo)}>{demo ? "Volver a datos reales" : "Ver datos de ejemplo"}</button></div>
    {demo && <div className="data-demo-banner" role="status"><strong>Vista de demostración</strong><span>Son cifras ficticias: no se guardan, no afectan rutas ni se mezclan con tus métricas.</span></div>}
    {!demo && unavailable && <div className="form-error" role="status">No pudimos cargar el resumen de 24 horas.</div>}
    {!demo && error && <div className="form-error" role="status">No pudimos cargar el historial agregado. La actividad individual sigue disponible.</div>}
    {!demo && loading && <div className="loading" role="status"><span className="spinner"/>Cargando métricas…</div>}
    <div className="data-kpis" aria-label="Historial disponible"><article><span>Tokens registrados</span><strong>{totalTokens === null ? "—" : number.format(totalTokens)}</strong><small>Entrada + salida</small></article><article><span>Entrada</span><strong>{view ? number.format(view.inputTokens) : "—"}</strong><small>Tokens reportados</small></article><article><span>Salida</span><strong>{view ? number.format(view.outputTokens) : "—"}</strong><small>Tokens reportados</small></article><article><span>Costo estimado</span><strong>{prices.value === null ? "—" : money.format(prices.value)}</strong><small>{prices.pricedRequests ? `${prices.pricedRequests} de ${prices.reportedRequests} solicitudes con uso tienen tarifa` : "Configura tarifas por modelo"}</small></article></div>
    <div className="data-insights-grid"><article className="data-panel data-trend"><div className="data-panel-heading"><div><h2>Uso de tokens</h2><p>Últimos 30 días · UTC</p></div><strong>{recent.length ? number.format(recent.reduce((sum, day) => sum + day.tokens, 0)) : "—"}</strong></div><div className="data-bars" role="img" aria-label="Barras diarias de tokens usados en los últimos 30 días">{recent.map(day => <div className="data-bar-slot" key={day.date} title={`${day.date}: ${number.format(day.tokens)} tokens`}><span style={{ height: `${day.tokens ? Math.max(4, day.tokens / max * 100) : 2}%` }} className={day.tokens ? "active" : ""}/></div>)}</div><div className="data-axis"><span>Hace 30 días</span><span>Hoy</span></div></article><article className="data-panel data-coverage"><h2>Calidad del dato</h2><div className="data-ring" style={{ "--ring-progress": `${view?.requests ? view.usageKnownRequests / view.requests * 100 : 0}%` } as React.CSSProperties}><strong>{view?.requests ? `${Math.round(view.usageKnownRequests / view.requests * 100)} %` : "—"}</strong><span>con uso reportado</span></div><p>{view ? `${number.format(view.usageKnownRequests)} de ${number.format(view.requests)} solicitudes tienen tokens de entrada y salida.` : "Aún no hay datos."}</p><p>Una respuesta sin conteo de tokens no se interpreta como consumo cero.</p></article></div>
    <div className="data-insights-grid"><article className="data-panel data-calendar-panel"><div className="data-panel-heading"><div><h2>Actividad diaria</h2><p>90 días · intensidad por solicitudes</p></div></div><div className="data-calendar" role="img" aria-label="Mapa de actividad de los últimos 90 días">{calendar.map(day => <span key={day.date} className={`level-${day.requests === 0 ? 0 : day.requests < 4 ? 1 : day.requests < 9 ? 2 : 3}`} title={`${day.date}: ${day.requests} solicitudes`}/>)}</div><div className="data-axis"><span>Hace 90 días</span><span>Hoy</span></div></article><article className="data-panel data-coverage"><h2>Disponibilidad observada</h2><strong className="data-observed-rate">{successRate}</strong><p>Éxito de solicitudes de las últimas 24 horas. No equivale al uptime: para medir uptime harán falta sondeos periódicos independientes del tráfico.</p><a href="#/activity">Ver actividad y errores →</a></article></div>
    <article className="data-panel data-model-breakdown"><div className="data-panel-heading"><div><h2>Consumo por modelo</h2><p>Tarifas actuales aplicadas al historial retenido; no son facturas del proveedor.</p></div></div>{view?.models.length ? <div className="catalog-table-wrap"><table><thead><tr><th>Modelo</th><th>Solicitudes</th><th>Entrada</th><th>Salida</th><th>Latencia media</th><th>Salida/s</th><th>Estimación</th></tr></thead><tbody>{view.models.map(row => { const model = visibleModels.find(item => modelName(item) === row.name); const price = priceOf(model); return <tr key={row.name}><td>{demo ? "Modelo de ejemplo" : model ? String((model.metadata as { displayName?: string }).displayName || row.name) : row.name}</td><td>{number.format(row.requests)}</td><td>{number.format(row.inputTokens)}</td><td>{number.format(row.outputTokens)}</td><td>{row.avgLatencyMillis == null ? "—" : `${Math.round(row.avgLatencyMillis)} ms`}</td><td>{row.outputTokensPerSecond == null ? "—" : `${row.outputTokensPerSecond.toLocaleString("es", { maximumFractionDigits: 1 })} tokens/s`}</td><td>{price ? money.format((row.inputTokens * price.inputPerMillion + row.outputTokens * price.outputPerMillion) / 1_000_000) : "Tarifa sin configurar"}</td></tr>; })}</tbody></table></div> : <p>Los modelos aparecerán cuando haya solicitudes con tokens reportados.</p>}</article>
    <p className="data-method">Las cifras abarcan solo eventos aún conservados. Si borras historial, los totales disminuyen. Los precios pueden cambiar y la estimación usa el valor configurado hoy; intentos de fallback sin tokens reportados pueden no reflejar todo el consumo facturado.</p>
  </section>;
}
