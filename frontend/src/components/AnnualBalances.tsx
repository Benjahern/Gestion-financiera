import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/api";
import type { Currency } from "../lib/types";
import { annualChange, exactMoney, minor, percent } from "../lib/metrics";
import { Button, EmptyState, Modal, SectionHeader, StatCard } from "./ui";

type Closing = { year: number; balance: string };
type History = { currency: Currency; current_year: number; items: Closing[] };
type Draft = { year: string; balance: string; editing: boolean };

export default function AnnualBalances({ currency, balance, hidden }: { currency: Currency; balance: string; hidden: boolean }) {
  const [history, setHistory] = useState<History | null>(null);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(true);
  const [retry, setRetry] = useState(0);
  const [draft, setDraft] = useState<Draft | null>(null);
  const [busy, setBusy] = useState(false);
  const [actionError, setActionError] = useState("");
  const [notice, setNotice] = useState("");
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setError("");
    api<History>(`/annual-balances?currency=${currency}`, { signal: controller.signal }).then(value => {
      if (!controller.signal.aborted) setHistory(value);
    }).catch(err => { if (!controller.signal.aborted) setError(err instanceof Error ? err.message : "No se pudieron cargar los cierres."); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [currency, retry]);
  const display = (value: bigint) => hidden ? "••••••" : exactMoney(value,currency);
  const signed = (value: bigint) => hidden ? "••••••" : `${value > 0n ? "+" : ""}${exactMoney(value,currency)}`;
  const save = async (event: FormEvent) => {
    event.preventDefault(); if (!draft || !history || busy) return;
    setActionError(""); setNotice("");
    const year = Number(draft.year);
    if (!/^\d{4}$/.test(draft.year) || year < 1900 || year >= history.current_year) { setActionError("Elige un año cerrado anterior al actual."); return; }
    const value = draft.balance.trim().replace(",", ".");
    try { if (!/^-?\d{1,12}(\.\d{1,2})?$/.test(value)) throw new Error(); minor(value,currency); } catch { setActionError("Introduce un saldo de hasta 12 dígitos sin separador de miles; CLP sin fracciones y USD con hasta dos decimales."); return; }
    if (!draft.editing && history.items.some(item => item.year === year) && !window.confirm(`Ya existe un cierre de ${year} en ${currency}. ¿Reemplazarlo?`)) return;
    setBusy(true);
    try {
      const saved = await api<Closing>(`/annual-balances/${year}`, { method: "PUT", body: { currency, balance: value } });
      setHistory(prev => prev && { ...prev, items: [...prev.items.filter(item => item.year !== year),saved].sort((a,b) => a.year-b.year) });
      setDraft(null); setNotice(`Cierre de ${year} guardado. Tus cuentas no cambiaron.`);
    } catch (err) { setActionError(err instanceof Error ? err.message : "No se pudo guardar el cierre."); }
    finally { setBusy(false); }
  };
  const remove = async (year: number) => {
    if (busy || !window.confirm(`¿Eliminar el cierre histórico de ${year} en ${currency}? Esto no elimina cuentas ni movimientos.`)) return;
    setBusy(true); setActionError(""); setNotice("");
    try {
      await api(`/annual-balances/${year}?currency=${currency}`, { method: "DELETE" });
      setHistory(prev => prev && { ...prev, items: prev.items.filter(item => item.year !== year) });
      setNotice(`Cierre de ${year} eliminado. Tus cuentas no cambiaron.`);
    } catch (err) { setActionError(err instanceof Error ? err.message : "No se pudo eliminar."); }
    finally { setBusy(false); }
  };
  const currentYear = history?.current_year ?? new Date().getUTCFullYear();
  const previous = new Map(history?.items.map(item => [item.year,minor(item.balance,currency)]));
  const current = minor(balance,currency);
  const change = annualChange(current,previous.get(currentYear-1));
  const rows = [...(history?.items ?? []).map(item => ({ year: item.year, value: minor(item.balance,currency), current: false })), { year: currentYear, value: current, current: true }];
  return <section className="panel annual-panel" aria-label="Comparación anual">
    <div className="annual-heading">
      <SectionHeader title="Comparación anual" subtitle={`Tu dinero total en ${currency} · cierres al 31 de diciembre`} />
      <Button className="secondary-button" disabled={loading || !!error || busy} onClick={() => { setActionError(""); setDraft({ year: String(currentYear-1), balance: "", editing: false }); }}>Añadir cierre anual</Button>
    </div>
    <p className="form-note">Copia de tu Excel el total de todas tus cuentas de esta moneda, incluyendo inversiones una sola vez. Estos cierres son referencias privadas: no crean movimientos ni cambian saldos.</p>
    {loading ? <EmptyState><span role="status">Cargando cierres anuales…</span></EmptyState> : error ? <><p className="error-message" role="alert">{error}</p><Button className="secondary-button" onClick={() => setRetry(n => n+1)}>Reintentar cierres</Button></> : <>
      <div className="stats-grid annual-stats">
        <StatCard label={`Cambio en ${currentYear} · a la fecha`} amount={change.difference === null ? "Sin comparación" : signed(change.difference)} note={change.difference === null ? `Añade el cierre de ${currentYear-1}` : `Frente al cierre de ${currentYear-1}; no depende del mes seleccionado`} icon="wallet" />
        <StatCard label="Variación del dinero total" amount={hidden ? "••••••" : change.difference === null ? "Sin datos" : percent(change.percentage,true)} note="(Saldo actual − cierre anterior) / cierre anterior × 100" icon="investment" />
        <StatCard label="Cierres registrados" amount={String(history?.items.length ?? 0)} note="CLP y USD se comparan por separado, sin tipo de cambio" icon="home" />
      </div>
      {!history?.items.length && <p className="form-note">Aún no hay cierres históricos. No inferimos años anteriores a partir del saldo inicial de tus cuentas.</p>}
      <div className="annual-table-scroll" tabIndex={0} aria-label="Tabla de cierres anuales">
        <table className="annual-table"><caption className="sr-only">Saldos y cambios anuales en {currency}</caption>
          <thead><tr><th>Año</th><th>Saldo total</th><th>Cambio en dinero</th><th>Variación anual</th><th>Acciones</th></tr></thead>
          <tbody>{rows.map(row => {
            const delta = annualChange(row.value,previous.get(row.year-1));
            return <tr key={row.year} className={row.current ? "annual-current" : undefined}>
              <th scope="row">{row.year}<small>{row.current ? "A la fecha · saldo actual" : "Cierre manual · 31 dic."}</small></th>
              <td>{display(row.value)}</td>
              <td>{delta.difference === null ? "—" : signed(delta.difference)}</td>
              <td>{delta.difference === null ? <small>Falta cierre {row.year-1}</small> : <>{hidden ? "••••••" : percent(delta.percentage,true)}{delta.percentage === null && <small>Base cero o negativa</small>}</>}</td>
              <td>{row.current ? <small>Calculado desde tus cuentas</small> : <div className="row-actions"><Button className="text-button" disabled={busy} aria-label={`Editar cierre ${row.year}`} onClick={() => { setActionError(""); setDraft({ year: String(row.year), balance: exactInput(row.value,currency), editing: true }); }}>Editar</Button><Button className="text-button danger" disabled={busy} aria-label={`Eliminar cierre ${row.year}`} onClick={() => void remove(row.year)}>Eliminar</Button></div>}</td>
            </tr>;
          })}</tbody>
        </table>
      </div>
      <p className="form-note">Esto mide cuánto aumentó o disminuyó tu dinero, no rentabilidad de inversiones: incluye ahorro, aportes y retiros. Sin cierre del año anterior no mostramos un porcentaje; con base cero o negativa, solo la diferencia monetaria.</p>
    </>}
    {notice && <p className="form-note" role="status">{notice}</p>}
    {actionError && !draft && <p className="error-message" role="alert">{actionError}</p>}
    {draft && <Modal title={draft.editing ? `Editar cierre ${draft.year}` : "Añadir cierre anual"} onClose={() => { if (!busy) setDraft(null); }}>
      <form onSubmit={save}>
        <fieldset disabled={busy} className="annual-form">
          <label className="field"><span>Año del cierre</span><input type="number" min="1900" max={currentYear-1} required value={draft.year} readOnly={draft.editing} onChange={e => setDraft({ ...draft, year: e.target.value })} /></label>
          <label className="field"><span>Saldo total al cierre · {currency}</span><input required inputMode="decimal" maxLength={17} autoComplete="off" value={draft.balance} onChange={e => setDraft({ ...draft, balance: e.target.value })} placeholder="Sin separador de miles" /></label>
          <p className="form-note">Total de dinero al 31 de diciembre de ese año, no ingresos ni aportes del año. Incluye inversiones una sola vez y únicamente {currency}. No se modifica ninguna cuenta.</p>
          {actionError && <p className="error-message" role="alert">{actionError}</p>}
          <div className="modal-actions"><Button className="secondary-button" onClick={() => setDraft(null)}>Cancelar</Button><Button className="primary-button" type="submit">{busy ? "Guardando…" : "Guardar cierre"}</Button></div>
        </fieldset>
      </form>
    </Modal>}
  </section>;
}

function exactInput(value: bigint, currency: Currency) {
  const sign = value < 0n ? "-" : "";
  const absolute = value < 0n ? -value : value;
  return currency === "CLP" ? String(value) : `${sign}${absolute/100n}.${String(absolute%100n).padStart(2,"0")}`;
}
