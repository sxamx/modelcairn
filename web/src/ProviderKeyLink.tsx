import { FormEvent, useEffect, useState } from "react";
import { api, APIError, Configuration, ConfigurationPlan, Resource, SecretMetadata } from "./api/client";

type Props = {
  providerId: string;
  accounts: Resource[];
  egresses: Resource[];
  onClose: () => void;
  onOpenSecrets: () => void;
  onSaved: () => Promise<void>;
};

const resourceName = (item: Resource): string => String(item.metadata?.name ?? "");
const resourceSpec = (item: Resource): Record<string, unknown> => ("spec" in item ? item.spec : {}) as Record<string, unknown>;
const reference = (name: string) => ({ name });
const changeLabel: Record<string, string> = { ProviderAccount: "Cuenta", Credential: "API key vinculada", Egress: "Salida de red" };
const operationLabel: Record<string, string> = { create: "Crear", update: "Actualizar", delete: "Eliminar" };
const resource = (kind: string, name: string, spec: object, displayName?: string) => ({
  kind, state: "present", metadata: { name, ...(displayName ? { displayName } : {}) }, spec,
});

export default function ProviderKeyLink({ providerId, accounts, egresses, onClose, onOpenSecrets, onSaved }: Props) {
  const providerAccounts = accounts.filter(item => (resourceSpec(item).providerRef as { name?: string } | undefined)?.name === providerId);
  const directEgresses = egresses.filter(item => resourceSpec(item).type === "direct" && resourceSpec(item).enabled !== false);
  const [secrets, setSecrets] = useState<SecretMetadata[]>([]);
  const [loading, setLoading] = useState(true);
  const [accountChoice, setAccountChoice] = useState(providerAccounts[0] ? resourceName(providerAccounts[0]) : "new");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [pending, setPending] = useState<{ configuration: Configuration; plan: ConfigurationPlan } | null>(null);

  useEffect(() => {
    let active = true;
    api.listSecrets().then(page => { if (active) setSecrets(page.items); })
      .catch(() => { if (active) setError("No pudimos cargar las claves guardadas."); })
      .finally(() => { if (active) setLoading(false); });
    return () => { active = false; };
  }, []);

  async function review(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const accountName = accountChoice === "new" ? String(fields.get("accountName") ?? "").trim() : accountChoice;
    const egressName = directEgresses.length ? String(fields.get("egress") ?? "") : `${providerId}-direct`;
    const credentialName = String(fields.get("credentialName") ?? "").trim();
    const secretName = String(fields.get("secret") ?? "");
    const additions = [
      ...(accountChoice === "new" ? [resource("ProviderAccount", accountName, { providerRef: reference(providerId) })] : []),
      ...(directEgresses.length ? [] : [resource("Egress", egressName, { type: "direct", enabled: true })]),
      resource("Credential", credentialName, {
        providerAccountRef: reference(accountName), egressRef: reference(egressName),
        secretRef: reference(secretName), enabled: true,
      }, String(fields.get("displayName") ?? "").trim()),
    ];
    const configuration = { apiVersion: "modelcairn.io/v1alpha1", kind: "Configuration", resources: additions } as Configuration;
    setBusy(true); setError("");
    try { setPending({ configuration, plan: await api.planConfiguration(configuration) }); }
    catch (reason) { setError(reason instanceof APIError && reason.status === 409 ? "Ya existe uno de esos identificadores. Elige otro." : "No pudimos revisar el vínculo. Comprueba la cuenta, la salida de red y la clave."); }
    finally { setBusy(false); }
  }

  async function apply() {
    if (!pending) return;
    setBusy(true); setError("");
    try {
      await api.applyConfiguration(pending.configuration, pending.plan.planToken);
      await onSaved();
      onClose();
    } catch {
      setPending(null);
      setError("No se pudo guardar el vínculo. La configuración pudo cambiar; revisa de nuevo antes de reintentar.");
    } finally { setBusy(false); }
  }

  return <div className="catalog-dialog-backdrop"><section className="catalog-dialog catalog-dialog-wide" role="dialog" aria-modal="true" aria-labelledby="link-key-title">
    <div className="catalog-dialog-head"><h2 id="link-key-title">Vincular API key</h2><button type="button" aria-label="Cerrar" disabled={busy} onClick={onClose}>×</button></div>
    {pending ? <><p>Confirma los cambios para vincular esta clave al proveedor. El valor del secreto no se mostrará.</p>
      <ul className="catalog-plan-list">{pending.plan.changes.map((change, index) => <li key={`${change.kind}-${change.name}-${index}`}><strong>{operationLabel[change.operation] ?? change.operation}</strong> {changeLabel[change.kind] ?? change.kind} · {change.name}</li>)}</ul>
      {error && <div className="form-error" role="alert">{error}</div>}
      <div className="catalog-dialog-actions"><button type="button" className="secondary" disabled={busy} onClick={() => setPending(null)}>Volver</button><button type="button" disabled={busy} onClick={apply}>{busy ? "Guardando…" : "Confirmar vínculo"}</button></div></> :
      <form onSubmit={review}><p>Elige una clave ya guardada. Después podrás usarla con los modelos de este proveedor.</p>
        {error && <div className="form-error" role="alert">{error}</div>}
        <label>Clave guardada<select name="secret" required defaultValue="" disabled={loading || !secrets.length}><option value="" disabled>Selecciona una clave</option>{secrets.map(item => <option key={item.name} value={item.name}>{item.name}</option>)}</select></label>
        {!loading && !secrets.length && <p className="catalog-hint">Todavía no hay claves guardadas. Guarda una y vuelve aquí para vincularla.</p>}
        <button type="button" className="catalog-text-button" onClick={onOpenSecrets}>Ir a claves guardadas</button>
        <div className="catalog-form-grid"><label>Identificador del vínculo<input name="credentialName" required pattern="[a-z][a-z0-9-]{0,62}" placeholder={`${providerId}-key`}/></label><label>Nombre visible<input name="displayName" maxLength={120} placeholder="Clave principal"/></label></div>
        <label>Cuenta del proveedor<select value={accountChoice} onChange={event => setAccountChoice(event.target.value)}>{providerAccounts.map(item => <option key={resourceName(item)} value={resourceName(item)}>{resourceName(item)}</option>)}<option value="new">Crear cuenta nueva</option></select></label>
        {accountChoice === "new" && <label>Identificador de la cuenta<input name="accountName" required pattern="[a-z][a-z0-9-]{0,62}" defaultValue={`${providerId}-account`}/></label>}
        {directEgresses.length ? <label>Salida de red<select name="egress" required>{directEgresses.map(item => <option key={resourceName(item)} value={resourceName(item)}>{resourceName(item)} · salida directa</option>)}</select></label> : <p className="catalog-hint">Se creará una salida directa en el nodo principal. Podrás configurar más salidas en una versión posterior.</p>}
        <div className="catalog-dialog-actions"><button type="button" className="secondary" onClick={onClose}>Cancelar</button><button disabled={busy || loading || !secrets.length}>{busy ? "Revisando…" : "Revisar vínculo"}</button></div>
      </form>}
  </section></div>;
}
