import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/api";
import { amount, calculate, emptySettings, units, validateSettings, type CalculatorSettings, type PlanningField } from "../lib/calculator";
import { money, type Account, type Currency } from "../lib/types";
import { Button } from "./ui";

export default function Calculator({ currency, accounts }: { currency: Currency; accounts: Account[] }) {
  const [settings, setSettings] = useState(() => emptySettings(currency));
  const [saved, setSaved] = useState("");
  const [income, setIncome] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setLoadError("");
    void api<{ settings: CalculatorSettings | null }>(`/calculator?currency=${currency}`, { signal: controller.signal })
      .then(({ settings: value }) => {
        if (controller.signal.aborted) return;
        const next = value ?? emptySettings(currency);
        setSettings(next); setSaved(JSON.stringify(next));
      })
      .catch(e => { if (!controller.signal.aborted) setLoadError(e.message); })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    return () => controller.abort();
  }, [currency, retry]);
  const dirty = !!saved && JSON.stringify(settings) !== saved;
  const edit = (key: PlanningField, value: string) => {
    setSettings(previous => ({ ...previous, [key]: value }));
    setNotice(""); setError("");
  };
  const save = async (event: FormEvent) => {
    event.preventDefault(); setError(""); setNotice("");
    try {
      validateSettings(settings);
      const canonical = { ...settings };
      for (const key of Object.keys(settings) as (keyof CalculatorSettings)[]) {
        if (key !== "currency") canonical[key] = amount(units(settings[key], key.endsWith("percent") ? "USD" : currency), key.endsWith("percent") ? "USD" : currency);
      }
      setBusy(true);
      const result = await api<{ settings: CalculatorSettings }>("/calculator", { method: "PUT", body: canonical });
      setSettings(result.settings); setSaved(JSON.stringify(result.settings));
      setNotice("Configuración guardada. No se modificó ninguna cuenta ni movimiento.");
    } catch (e) { setError(e instanceof Error ? e.message : "No se pudo guardar"); }
    finally { setBusy(false); }
  };
  let result: ReturnType<typeof calculate> | null = null;
  let calculationNote = "Completa la configuración e introduce un ingreso para ver el reparto.";
  try { result = calculate(settings, income); }
  catch (e) { if (income.trim() && e instanceof Error) calculationNote = e.message; }
  const field = (key: PlanningField, label: string, percent = false) => (
    <label className="field" key={key}>
      <span>{label} · {percent ? "%" : currency}</span>
      <input inputMode="decimal" autoComplete="off" value={settings[key]} required disabled={busy}
        maxLength={16} onChange={e => edit(key, e.target.value)} />
    </label>
  );
  const display = (value: bigint) => money(amount(value, currency), currency);
  if (loading) return <div className="page" role="status">Cargando calculadora…</div>;
  if (loadError) return <div className="page"><p className="error-message" role="alert">{loadError}</p><Button className="secondary-button" onClick={() => setRetry(n => n + 1)}>Reintentar</Button></div>;
  return (
    <div className="page calculator-page">
      <div className="page-intro"><div>
        <div className="eyebrow">De tu Excel a tu planificación</div>
        <h1 className="page-title">Calculadora mensual</h1>
        <p className="page-subtitle">Reparte un ingreso según tus prioridades, sin crear movimientos ni cambiar saldos.</p>
      </div><span className="calculator-badge">{currency} · {dirty ? "Cambios sin guardar" : "Configuración privada"}</span></div>
      <form onSubmit={save} className="calculator-config">
        <fieldset disabled={busy} className="panel calculator-fieldset">
          <legend>1 · Tu reparto de referencia</legend>
          <p className="form-note">Introduce los importes de un sueldo de referencia. Calcularemos sus proporciones para repartir cualquier ingreso. Puedes usar 0 en un destino. Sin separadores de miles.</p>
          <div className="calculator-fields">
            {field("reference_income", "Sueldo de referencia")}
            {field("general", "Gastos generales")}
            {field("wants", "Caprichos")}
            {field("investment", "Inversión total")}
          </div>
          <h2 className="calculator-subheading">Dentro de la inversión</h2>
          <div className="calculator-fields">{field("index_fund", "Fondo indexado")}{field("reserve", "Reserva para emergencia y metas")}</div>
          <p className="form-note">Los tres destinos deben sumar el sueldo. Fondo indexado + reserva deben sumar la inversión total.</p>
        </fieldset>
        <fieldset disabled={busy} className="panel calculator-fieldset">
          <legend>2 · Primero, tu fondo de emergencia</legend>
          <div className="calculator-fields">
            {field("emergency_goal", "Meta del fondo")}{field("emergency_balance", "Saldo actual del fondo")}
            <label className="field"><span>Copiar saldo de una cuenta · opcional</span>
              <select value="" onChange={e => {
                const account = accounts.find(a => String(a.id) === e.target.value && a.currency === currency);
                if (!account) return;
                try { units(account.balance, currency); edit("emergency_balance", account.balance); }
                catch { setError("El fondo necesita un saldo no negativo. Introduce su valor manualmente."); }
              }}><option value="">Elegir cuenta {currency}</option>
                {accounts.filter(a => a.currency === currency).map(a => <option key={a.id} value={a.id}>{a.name}</option>)}
              </select>
            </label>
          </div>
          <p className="form-note">El saldo es una referencia manual, no un vínculo automático con la cuenta. Hasta completar la meta, toda la reserva va a emergencia. Como en el Excel, el destino depende del saldo actual: no se redistribuye el excedente del aporte que alcance la meta.</p>
          <h2 className="calculator-subheading">Cuando la meta esté completa</h2>
          <p className="form-note">La reserva deja de ir a emergencia y se divide entre estos destinos. Los porcentajes deben sumar 100%.</p>
          <div className="calculator-fields">{field("own_home_percent", "Casa propia", true)}{field("rental_percent", "Casa de inversión", true)}{field("travel_percent", "Viaje", true)}</div>
        </fieldset>
        {error && <p className="error-message" role="alert">{error}</p>}
        {notice && <p role="status">{notice}</p>}
        <div className="calculator-save"><p className="form-note">Se guarda una configuración por moneda, compartida entre tus dispositivos. El ingreso simulado no se guarda.</p>
          <Button type="submit" className="primary-button" disabled={busy}>{busy ? "Guardando…" : "Guardar configuración"}</Button></div>
      </form>
      <section className="panel calculator-simulation" aria-labelledby="simulation-title">
        <div><div className="eyebrow">Solo simulación</div><h2 id="simulation-title">3 · ¿Cuánto quieres repartir?</h2></div>
        <div className="calculator-simulation-input">
          <label className="field"><span>Ingreso a repartir · {currency}</span><input inputMode="decimal" autoComplete="off" maxLength={16} value={income} onChange={e => setIncome(e.target.value)} /></label>
          <Button className="secondary-button" onClick={() => setIncome(settings.reference_income)}>Usar sueldo de referencia</Button>
        </div>
        {!result ? <p className="form-note" role="status">{calculationNote}</p> : <>
          <div className="calculator-progress">
            <span>Fondo de emergencia · {result.complete ? "Meta completa" : "En construcción"}</span><strong>{new Intl.NumberFormat("es-CL", { maximumFractionDigits: 2 }).format(result.progress)}%</strong>
            <progress max="100" value={Math.min(result.progress, 100)} aria-label="Progreso del fondo de emergencia" />
          </div>
          <div className="calculator-results" aria-label="Reparto del ingreso" aria-live="polite">
            {([["Gastos generales", result.general], ["Caprichos", result.wants], ["Fondo indexado", result.index], ["Fondo de emergencia", result.emergency], ["Casa propia", result.ownHome], ["Casa de inversión", result.rental], ["Viaje", result.travel]] as const).map(([label, value]) => <div className="calculator-result" key={label}><span>{label}</span><strong>{display(value)}</strong></div>)}
          </div>
          <div className="calculator-total"><span>Total repartido</span><strong>{display(result.income)}</strong></div>
          <p className="form-note">Inversión incluida en el reparto: {display(result.investment)}. Redondeo a pesos o centavos conservando el total exacto. No se ejecutan transferencias ni compras.</p>
        </>}
      </section>
    </div>
  );
}
