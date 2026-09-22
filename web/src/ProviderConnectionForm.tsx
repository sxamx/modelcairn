import { FormEvent, useState } from "react";
import { api, APIError, Resource } from "./api/client";

export default function ProviderConnectionForm({ providerId, onClose, onSaved }: { providerId: string; onClose: () => void; onSaved: () => Promise<void> }) {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function save(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const fields = new FormData(event.currentTarget);
    const name = String(fields.get("name") ?? "").trim();
    const baseUrl = String(fields.get("baseUrl") ?? "").trim();
    setBusy(true); setError("");
    try {
      await api.createResource("provider-connections", {
        kind: "ProviderConnection", state: "present", metadata: { name },
        spec: { providerRef: { name: providerId }, baseUrl, adapter: "openai-chat-v1", allowPrivateNetwork: false, enabled: true },
      } as Resource);
      await onSaved();
      onClose();
    } catch (reason) {
      setError(reason instanceof APIError && reason.status === 409 ? "Ese identificador ya existe. Elige otro." : "No se pudo guardar la conexión. Comprueba que la URL es HTTPS y corresponde a la API del proveedor.");
    } finally { setBusy(false); }
  }

  return <div className="catalog-dialog-backdrop"><form className="catalog-dialog" onSubmit={save} role="dialog" aria-modal="true" aria-labelledby="new-connection-title">
    <div className="catalog-dialog-head"><h2 id="new-connection-title">Añadir conexión API</h2><button type="button" aria-label="Cerrar" disabled={busy} onClick={onClose}>×</button></div>
    <p>Indica la URL base compatible con Chat Completions. Después podrás añadir los modelos de esta conexión.</p>
    {error && <div className="form-error" role="alert">{error}</div>}
    <label>Identificador interno<input name="name" required pattern="[a-z][a-z0-9-]{0,62}" placeholder={`${providerId}-api`}/></label>
    <label>URL base de la API<input name="baseUrl" type="url" required maxLength={2048} placeholder="https://api.ejemplo.com/v1"/></label>
    <small>La conexión usa el adaptador OpenAI Chat Completions. Las URLs privadas se configuran en Opciones avanzadas.</small>
    <div className="catalog-dialog-actions"><button type="button" className="secondary" disabled={busy} onClick={onClose}>Cancelar</button><button disabled={busy}>{busy ? "Guardando…" : "Guardar conexión"}</button></div>
  </form></div>;
}
