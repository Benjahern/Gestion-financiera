import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/api";
import { today, type Account, type Currency, type Kind, type Movement } from "../lib/types";
import { Button, Modal } from "./ui";
import CategoryPicker from "./CategoryPicker";

type Common = { onClose: () => void; onSaved: () => Promise<void> };
const errorText = (error: unknown) =>
  error instanceof Error ? error.message : "No se pudo guardar.";

export function AccountForm({
  account,
  currency,
  onClose,
  onSaved,
}: Common & { account?: Account; currency: Currency }) {
  const [name, setName] = useState(account?.name ?? "");
  const [type, setType] = useState(account?.type ?? "");
  const [unit, setUnit] = useState<Currency>(account?.currency ?? currency);
  const [balance, setBalance] = useState(account?.opening_balance ?? "0");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(account ? `/accounts/${account.id}` : "/accounts", {
        method: account ? "PUT" : "POST",
        body: {
          name: name.trim(),
          type: type.trim(),
          currency: unit,
          opening_balance: balance.trim().replace(",", "."),
        },
      });
      await onSaved();
      onClose();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      title={account ? "Editar cuenta" : "Añadir cuenta"}
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <form onSubmit={submit}>
        <div className="form-grid">
          <label className="field full">
            <span>Nombre de la cuenta</span>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              required
              maxLength={100}
              autoFocus
            />
          </label>
          <label className="field">
            <span>Tipo de cuenta</span>
            <input
              value={type}
              onChange={(e) => setType(e.target.value)}
              required
              maxLength={100}
              placeholder="Banco, efectivo, inversión…"
            />
          </label>
          <label className="field">
            <span>Moneda</span>
            <select value={unit} onChange={(e) => setUnit(e.target.value as Currency)}>
              <option value="CLP">Pesos chilenos (CLP)</option>
              <option value="USD">Dólares (USD)</option>
            </select>
          </label>
          <label className="field full">
            <span>Saldo inicial ({unit})</span>
            <input
              inputMode="decimal"
              value={balance}
              onChange={(e) => setBalance(e.target.value)}
              required
              maxLength={16}
            />
          </label>
        </div>
        <p className="form-note">
          El saldo actual se calcula desde el saldo inicial y tus movimientos.{" "}
          {account &&
            "Cambiar el saldo inicial modifica el saldo calculado. No puedes cambiar la moneda si hay movimientos asociados."}
        </p>
        {error && (
          <div role="alert" className="error-message">
            {error}
          </div>
        )}
        <div className="modal-actions">
          <Button className="secondary-button" disabled={busy} onClick={onClose}>
            Cancelar
          </Button>
          <Button className="primary-button" type="submit" disabled={busy}>
            {busy ? "Guardando…" : "Guardar cuenta"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export function MovementForm({
  movement,
  kind,
  accounts,
  currency,
  onClose,
  onSaved,
}: Common & {
  movement?: Movement;
  kind: Kind;
  accounts: Account[];
  currency: Currency;
}) {
  const [title, setTitle] = useState(
    movement?.title === "Pendiente de completar" ? "" : (movement?.title ?? ""),
  );
  const [amount, setAmount] = useState(movement?.amount ?? "");
  const [category, setCategory] = useState(
    movement?.category === "Sin categoría" ? "" : (movement?.category ?? ""),
  );
  const [confirmed, setConfirmed] = useState(movement?.category_confirmed ?? false);
  const [categories, setCategories] = useState<{ id: number; name: string }[]>([]);
  const [categoriesLoading, setCategoriesLoading] = useState(true);
  const [categoriesError, setCategoriesError] = useState(false);
  const [categoryRetry, setCategoryRetry] = useState(0);
  const [predictionStatus, setPredictionStatus] = useState<"idle" | "loading" | "ready" | "error">(
    "idle",
  );
  const [prediction, setPrediction] = useState<{
    category: string;
    confidence: number;
    automatic: boolean;
    examples: number;
  } | null>(null);
  const [categoryTouched, setCategoryTouched] = useState(!!movement && !movement.pending);
  useEffect(() => {
    if (kind !== "expense") return;
    const controller = new AbortController();
    setCategoriesLoading(true);
    setCategoriesError(false);
    void api<{ id: number; name: string }[]>("/categories", { signal: controller.signal })
      .then((items) => {
        if (!controller.signal.aborted) {
          setCategories(items);
          setCategoriesLoading(false);
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setCategoriesError(true);
          setCategoriesLoading(false);
        }
      });
    return () => controller.abort();
  }, [kind, categoryRetry]);
  useEffect(() => {
    if (kind !== "expense" || !title.trim()) {
      setPrediction(null);
      setPredictionStatus("idle");
      return;
    }
    const controller = new AbortController();
    setPredictionStatus("loading");
    const timer = window.setTimeout(() => {
      void api<{ category: string; confidence: number; automatic: boolean; examples: number }>(
        "/classifier/predict",
        { method: "POST", body: { description: title.trim() }, signal: controller.signal },
      )
        .then((p) => {
          if (controller.signal.aborted) return;
          setPrediction(p);
          setPredictionStatus("ready");
          if (!categoryTouched) {
            setCategory(p.automatic ? p.category : "");
            setConfirmed(false);
          }
        })
        .catch(() => {
          if (!controller.signal.aborted) {
            setPrediction(null);
            setPredictionStatus("error");
          }
        });
    }, 350);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [title, kind, categoryTouched, movement]);
  const [date, setDate] = useState(movement?.date ?? today());
  const [accountID, setAccountID] = useState(
    String(
      movement?.account_id ??
        accounts.find((a) => a.currency === currency && (kind !== "investment" || a.is_investment))
          ?.id ??
        (kind === "investment" ? undefined : accounts[0]?.id) ??
        "",
    ),
  );
  const [sourceAccountID, setSourceAccountID] = useState(String(movement?.source_account_id ?? ""));
  const [investmentMode, setInvestmentMode] = useState<"legacy_outflow" | "contribution">(
    movement?.kind === "investment"
      ? (movement.investment_mode ?? "legacy_outflow")
      : "contribution",
  );
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const unit = accounts.find((a) => String(a.id) === accountID)?.currency ?? currency;
  const investmentDestinations = accounts.filter((a) => a.currency === currency && a.is_investment);
  const selectedDestination = accounts.find((a) => String(a.id) === accountID);
  const investmentSources = accounts.filter(
    (a) => a.currency === selectedDestination?.currency && String(a.id) !== accountID,
  );
  const label =
    kind === "income" ? "ingreso" : kind === "expense" ? "gasto" : "aporte de inversión";
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setBusy(true);
    setError("");
    try {
      await api(movement ? `/movements/${movement.id}` : "/movements", {
        method: movement ? "PUT" : "POST",
        body: {
          title: title.trim(),
          category: category.trim() || "Sin categoría",
          category_confirmed: kind === "expense" && confirmed && !!category.trim(),
          account_id: Number(accountID),
          ...(kind === "investment"
            ? {
                investment_mode: investmentMode,
                source_account_id:
                  investmentMode === "contribution" && sourceAccountID
                    ? Number(sourceAccountID)
                    : null,
              }
            : {}),
          date,
          amount: amount.trim().replace(",", "."),
          kind,
        },
      });
      await onSaved();
      onClose();
    } catch (err) {
      setError(errorText(err));
    } finally {
      setBusy(false);
    }
  };
  return (
    <Modal
      title={`${movement ? "Editar" : "Añadir"} ${label}`}
      onClose={() => {
        if (!busy) onClose();
      }}
    >
      <form onSubmit={submit}>
        <div className="form-grid">
          <label className="field full">
            <span>{kind === "expense" ? "Qué compraste" : "Concepto"}</span>
            <input
              value={title}
              onChange={(e) => {
                setTitle(e.target.value);
                setPrediction(null);
                setConfirmed(false);
                if (!categoryTouched) setCategory("");
              }}
              required
              maxLength={150}
              autoFocus
            />
          </label>
          {kind === "investment" && investmentMode === "contribution" && (
            <label className="field">
              <span>Origen del dinero</span>
              <select value={sourceAccountID} onChange={(e) => setSourceAccountID(e.target.value)}>
                <option value="">Dinero externo · no descontar otra cuenta</option>
                {investmentSources.map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name} · {a.currency}
                  </option>
                ))}
              </select>
            </label>
          )}
          <label className="field">
            <span>
              {kind === "investment"
                ? investmentMode === "contribution"
                  ? "Destino de inversión"
                  : "Cuenta que se descuenta · registro anterior"
                : "Cuenta"}
            </span>
            <select
              value={accountID}
              onChange={(e) => {
                setAccountID(e.target.value);
                if (sourceAccountID === e.target.value) setSourceAccountID("");
              }}
              required
            >
              <option value="" disabled>
                Selecciona una cuenta
              </option>
              {(kind === "investment" && investmentMode === "contribution"
                ? investmentDestinations
                : accounts.filter((a) => a.currency === currency)
              ).map((a) => (
                <option key={a.id} value={a.id}>
                  {a.name} · {a.currency}
                </option>
              ))}
            </select>
          </label>
          {kind === "investment" && movement?.investment_mode === "legacy_outflow" && (
            <div className="field full">
              <span>Este aporte antiguo sigue descontándose de la cuenta seleccionada.</span>
              <Button
                className="text-button"
                onClick={() => {
                  setInvestmentMode("contribution");
                  const destination = investmentDestinations[0];
                  if (destination) setAccountID(String(destination.id));
                }}
              >
                Cambiar a aporte en una cuenta de inversión
              </Button>
            </div>
          )}
          <label className="field">
            <span>Importe ({unit})</span>
            <input
              inputMode="decimal"
              value={amount}
              onChange={(e) => setAmount(e.target.value)}
              required
              maxLength={16}
              placeholder={unit === "CLP" ? "0" : "0,00"}
            />
          </label>
          <label className="field">
            <span>Fecha</span>
            <input type="date" value={date} onChange={(e) => setDate(e.target.value)} required />
          </label>
          {kind === "expense" ? (
            <CategoryPicker
              value={category}
              categories={categories}
              loading={categoriesLoading}
              error={categoriesError}
              onRetry={() => setCategoryRetry((value) => value + 1)}
              onChange={(value) => {
                setCategory(value);
                setCategoryTouched(true);
                setConfirmed(!!value);
              }}
            />
          ) : (
            <label className="field">
              <span>Categoría</span>
              <input
                value={category}
                onChange={(e) => {
                  setCategory(e.target.value);
                  setCategoryTouched(true);
                  setConfirmed(true);
                }}
                required
                maxLength={100}
              />
            </label>
          )}
        </div>
        {kind === "expense" && (
          <div className="classifier-help">
            {movement?.pending && (
              <p>Este importe ya cuenta como gasto. Completarlo no crea otra compra.</p>
            )}
            <div className="classifier-status" role="status" aria-live="polite">
              {predictionStatus === "loading" ? (
                <p>Buscando coincidencias en tus gastos confirmados…</p>
              ) : predictionStatus === "error" ? (
                <p>
                  No pudimos consultar las sugerencias. Puedes elegir una categoría y guardar
                  normalmente.
                </p>
              ) : prediction?.category ? (
                <p>
                  {prediction.automatic ? "Categoría automática" : "Sugerencia"}:{" "}
                  <strong>{prediction.category}</strong> · basada en {prediction.examples} ejemplos
                  confirmados.{" "}
                  {!prediction.automatic && (
                    <Button
                      className="text-button"
                      onClick={() => {
                        setCategory(prediction.category);
                        setCategoryTouched(true);
                        setConfirmed(true);
                      }}
                    >
                      Usar categoría
                    </Button>
                  )}
                </p>
              ) : !title.trim() ? (
                <p>Escribe qué compraste para buscar una sugerencia.</p>
              ) : prediction && prediction.examples > 0 ? (
                <p>
                  No hay coincidencias con tus {prediction.examples} ejemplos confirmados. Elige una
                  categoría para enseñarle esta descripción.
                </p>
              ) : (
                <p>
                  <strong>Tu clasificador está empezando.</strong> Elige una categoría y confirma el
                  aprendizaje. Al guardar, tendrá su primer ejemplo; podrá sugerirla en
                  descripciones con palabras en común.
                </p>
              )}
            </div>
            <label className="classifier-confirm">
              <input
                type="checkbox"
                checked={confirmed}
                disabled={!category}
                onChange={(e) => setConfirmed(e.target.checked)}
              />{" "}
              Aprender de esta categoría al guardar el gasto.
            </label>
          </div>
        )}
        <p className="form-note">
          {unit === "CLP"
            ? "Los pesos se registran sin decimales."
            : "Los dólares admiten hasta dos decimales."}{" "}
          {kind === "investment" &&
            (investmentMode === "contribution"
              ? "El dinero se suma al destino. Si eliges una cuenta de origen, se descuenta de ella; con dinero externo, no se descuenta otra cuenta. No representa una valoración de mercado."
              : "Este registro anterior descuenta dinero de la cuenta seleccionada; no representa una valoración de mercado.")}
        </p>
        {error && (
          <div role="alert" className="error-message">
            {error}
          </div>
        )}
        <div className="modal-actions">
          <Button className="secondary-button" disabled={busy} onClick={onClose}>
            Cancelar
          </Button>
          <Button
            className="primary-button"
            type="submit"
            disabled={
              busy ||
              (kind === "investment" && investmentMode === "contribution"
                ? investmentDestinations.length === 0
                : accounts.length === 0)
            }
          >
            {busy ? "Guardando…" : "Guardar movimiento"}
          </Button>
        </div>
      </form>
    </Modal>
  );
}

export default MovementForm;
