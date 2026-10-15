import { useState, type FormEvent } from "react";
import { api } from "../lib/api";
import { money, type Account, type Currency } from "../lib/types";
import { Button, EmptyState, SectionHeader } from "./ui";

export default function InvestmentAccounts({
  accounts,
  currency,
  onAccountsChanged,
}: {
  accounts: Account[];
  currency: Currency;
  onAccountsChanged: () => Promise<void>;
}) {
  const [accountID, setAccountID] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const selected = accounts.filter((a) => a.currency === currency && a.is_investment);
  const available = accounts.filter((a) => a.currency === currency && !a.is_investment);
  const validSelection = available.some((a) => String(a.id) === accountID);
  const change = async (id: number, isInvestment: boolean) => {
    setBusy(true);
    setError("");
    try {
      await api(`/accounts/${id}/investment`, {
        method: "PUT",
        body: { is_investment: isInvestment },
      });
      await onAccountsChanged();
      setAccountID("");
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo actualizar la cuenta.");
    } finally {
      setBusy(false);
    }
  };
  const submit = (event: FormEvent) => {
    event.preventDefault();
    if (validSelection && !busy) void change(Number(accountID), true);
  };
  return (
    <section className="panel" aria-label="Mis cuentas de inversión" aria-busy={busy}>
      <SectionHeader
        title="Mis cuentas de inversión"
        subtitle={`Saldos actuales en ${currency}, independientes del mes seleccionado.`}
      />
      <p className="form-note">
        Añade aquí las cuentas que ya creaste para identificarlas como inversión. Añadir o quitar
        una cuenta de esta sección no cambia su saldo ni crea movimientos.
      </p>
      {selected.length === 0 ? (
        <EmptyState>Aún no has seleccionado cuentas de inversión en {currency}.</EmptyState>
      ) : (
        <div className="accounts-grid">
          {selected.map((account) => (
            <div className="account-card" key={account.id}>
              <div className="account-card-type">{account.type}</div>
              <div className="account-card-name">{account.name}</div>
              <div className="account-card-bottom">
                <strong>{money(account.balance, account.currency)}</strong>
                <span>{account.currency}</span>
              </div>
              <Button
                className="text-button"
                disabled={busy}
                onClick={() => void change(account.id, false)}
                aria-label={`Quitar ${account.name} de inversiones sin borrar la cuenta`}
              >
                Quitar de inversiones
              </Button>
            </div>
          ))}
        </div>
      )}
      <form onSubmit={submit}>
        <label className="field full">
          <span>Añadir una cuenta existente ({currency})</span>
          <select
            value={validSelection ? accountID : ""}
            onChange={(event) => setAccountID(event.target.value)}
            disabled={busy || available.length === 0}
            required
          >
            <option value="" disabled>
              Selecciona una cuenta
            </option>
            {available.map((account) => (
              <option key={account.id} value={account.id}>
                {account.name} · {account.currency}
              </option>
            ))}
          </select>
        </label>
        {available.length === 0 && (
          <p className="form-note">
            No hay otras cuentas en {currency}. Puedes crearlas en Cuentas.
          </p>
        )}
        {error && (
          <div className="error-message" role="alert">
            {error}
          </div>
        )}
        <div className="modal-actions">
          <Button className="secondary-button" type="submit" disabled={busy || !validSelection}>
            {busy ? "Guardando…" : "Añadir a inversiones"}
          </Button>
        </div>
      </form>
    </section>
  );
}
