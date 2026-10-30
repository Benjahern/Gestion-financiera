import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/api";
import {
  amount,
  calculate,
  emptySettings,
  units,
  validateSettings,
  type CalculatorSettings,
  type PlanningField,
} from "../lib/calculator";
import { money, type Account, type Currency } from "../lib/types";
import { Button } from "./ui";

export default function Calculator({
  currency,
  accounts,
}: {
  currency: Currency;
  accounts: Account[];
}) {
  const [settings, setSettings] = useState(() => emptySettings(currency));
  const [saved, setSaved] = useState("");
  const [configured, setConfigured] = useState(false);
  const [income, setIncome] = useState("");
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true);
    setLoadError("");
    void api<{ settings: CalculatorSettings | null }>(`/calculator?currency=${currency}`, {
      signal: controller.signal,
    })
      .then(({ settings: value }) => {
        if (controller.signal.aborted) return;
        const next = {
          ...(value ?? emptySettings(currency)),
          emergency_goal: value?.emergency_goal ?? (currency === "CLP" ? "300000" : ""),
        };
        setConfigured(value !== null);
        setSettings(next);
        setSaved(JSON.stringify(next));
      })
      .catch((e) => {
        if (!controller.signal.aborted) setLoadError(e.message);
      })
      .finally(() => {
        if (!controller.signal.aborted) setLoading(false);
      });
    return () => controller.abort();
  }, [currency, retry]);
  const dirty = !!saved && JSON.stringify(settings) !== saved;
  const edit = (key: PlanningField, value: string) => {
    setSettings((previous) => ({ ...previous, [key]: value }));
    setNotice("");
    setError("");
  };
  const save = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    setNotice("");
    try {
      validateSettings(settings);
      const canonical = { ...settings };
      for (const key of Object.keys(settings) as (keyof CalculatorSettings)[]) {
        if (key !== "currency")
          canonical[key] = amount(
            units(settings[key], key.endsWith("percent") ? "USD" : currency),
            key.endsWith("percent") ? "USD" : currency,
          );
      }
      setBusy(true);
      const result = await api<{ settings: CalculatorSettings }>("/calculator", {
        method: "PUT",
        body: canonical,
      });
      setSettings(result.settings);
      setSaved(JSON.stringify(result.settings));
      setConfigured(true);
      setNotice("Configuración guardada. No se modificó ninguna cuenta ni movimiento.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo guardar");
    } finally {
      setBusy(false);
    }
  };
  let result: ReturnType<typeof calculate> | null = null;
  let calculationNote = configured
    ? "Introduce el dinero que quieres repartir."
    : "No hay reglas del Excel guardadas para esta moneda.";
  if (configured) {
    try {
      result = calculate(settings, income);
    } catch (e) {
      if (income.trim() && e instanceof Error) calculationNote = e.message;
    }
  }
  const field = (key: PlanningField, label: string, percent = false) => (
    <label className="field" key={key}>
      <span>
        {label} · {percent ? "%" : currency}
      </span>
      <input
        inputMode="decimal"
        autoComplete="off"
        value={settings[key]}
        required
        disabled={busy}
        maxLength={16}
        onChange={(e) => edit(key, e.target.value)}
      />
    </label>
  );
  const display = (value: bigint) => money(amount(value, currency), currency);
  if (loading)
    return (
      <div className="page" role="status">
        Cargando calculadora…
      </div>
    );
  if (loadError)
    return (
      <div className="page">
        <p className="error-message" role="alert">
          {loadError}
        </p>
        <Button className="secondary-button" onClick={() => setRetry((n) => n + 1)}>
          Reintentar
        </Button>
      </div>
    );
  return (
    <div className="page calculator-page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Como en tu Excel</div>
          <h1 className="page-title">Calculadora mensual</h1>
          <p className="page-subtitle">
            Introduce solo el dinero a repartir. Las reglas del Excel se mantienen fijas; no se
            crean movimientos ni se cambian saldos.
          </p>
        </div>
        <span className="calculator-badge">
          {currency} · {dirty ? "Meta sin guardar" : "Reglas fijas"}
        </span>
      </div>
      <section className="panel calculator-simulation" aria-labelledby="simulation-title">
        <div>
          <div className="eyebrow">Solo simulación</div>
          <h2 id="simulation-title">¿Cuánto quieres repartir?</h2>
        </div>
        <div className="calculator-simulation-input">
          <label className="field">
            <span>Dinero a repartir · {currency}</span>
            <input
              inputMode="decimal"
              autoComplete="off"
              maxLength={16}
              value={income}
              onChange={(e) => setIncome(e.target.value)}
            />
          </label>
        </div>
        {!result ? (
          <p className="form-note" role="status">
            {calculationNote}
          </p>
        ) : (
          <>
            <div className="calculator-progress">
              <span>
                Fondo de emergencia · {result.complete ? "Meta completa" : "En construcción"}
              </span>
              <strong>
                {new Intl.NumberFormat("es-CL", { maximumFractionDigits: 2 }).format(
                  result.progress,
                )}
                %
              </strong>
              <progress
                max="100"
                value={Math.min(result.progress, 100)}
                aria-label="Progreso del fondo de emergencia"
              />
            </div>
            <div className="calculator-results" aria-label="Reparto del ingreso" aria-live="polite">
              {(
                [
                  ["Gastos generales", result.general],
                  ["Caprichos", result.wants],
                  ["Fondo indexado", result.index],
                  ["Fondo de emergencia", result.emergency],
                  ["Casa propia", result.ownHome],
                  ["Casa de inversión", result.rental],
                  ["Viaje", result.travel],
                ] as const
              ).map(([label, value]) => (
                <div className="calculator-result" key={label}>
                  <span>{label}</span>
                  <strong>{display(value)}</strong>
                </div>
              ))}
            </div>
            <div className="calculator-total">
              <span>Total repartido</span>
              <strong>{display(result.income)}</strong>
            </div>
            <p className="form-note">
              Inversión incluida en el reparto: {display(result.investment)}. Redondeo a pesos o
              centavos conservando el total exacto. No se ejecutan transferencias ni compras.
            </p>
          </>
        )}
      </section>
      <details className="panel calculator-fieldset">
        <summary>
          Editar meta del fondo de emergencia · actual: {settings.emergency_goal || "sin definir"}{" "}
          {currency}
        </summary>
        <form onSubmit={save} className="calculator-config">
          <p className="form-note">
            La meta inicial es $300.000 CLP. La puedes cambiar cuando lo necesites; las demás reglas
            permanecen iguales.
          </p>
          <div className="calculator-fields">{field("emergency_goal", "Meta del fondo")}</div>
          {error && (
            <p className="error-message" role="alert">
              {error}
            </p>
          )}
          {notice && <p role="status">{notice}</p>}
          <div className="calculator-save">
            <p className="form-note">
              La configuración se guarda de forma privada y no modifica cuentas ni movimientos.
            </p>
            <Button type="submit" className="primary-button" disabled={busy}>
              {busy ? "Guardando…" : "Guardar meta"}
            </Button>
          </div>
        </form>
      </details>
    </div>
  );
}
