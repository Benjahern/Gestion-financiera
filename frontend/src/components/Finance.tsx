import { useEffect, useState } from "react";
import { api } from "../lib/api";
import CategoryPicker from "./CategoryPicker";
import InvestmentAccounts from "./InvestmentAccounts";
import InvestmentOverview from "./InvestmentOverview";
import AnnualBalances from "./AnnualBalances";
import {
  money,
  dateLabel,
  monthLabel,
  pageConfig,
  type Account,
  type Currency,
  type Movement,
  type MovementPage,
  type Overview,
  type Page,
} from "../lib/types";
import { Button, EmptyState, Icon, SectionHeader, StatCard } from "./ui";

export function MovementRow({
  item,
  onEdit,
  onDelete,
}: {
  item: Movement;
  onEdit?: (item: Movement) => void;
  onDelete?: (item: Movement) => void;
}) {
  return (
    <div className="movement-row">
      <div className={`movement-icon ${item.kind}`}>
        <Icon name={item.kind} size={18} />
      </div>
      <div className="movement-primary">
        <strong>{item.title}</strong>
        {item.pending && (
          <span className="pending-label">Pendiente · completar descripción o categoría</span>
        )}
        <span>
          {item.category} · {item.account}
        </span>
      </div>
      <span className="movement-date">{dateLabel(item.date)}</span>
      <strong className={`movement-amount ${item.kind}`}>
        {item.kind === "income" ? "+" : "−"}
        {money(item.amount, item.currency)}
      </strong>
      {onEdit && onDelete && (
        <div className="row-actions">
          <Button
            className="text-button"
            aria-label={`Editar ${item.title}`}
            onClick={() => onEdit(item)}
          >
            Editar
          </Button>
          <Button
            className="text-button danger"
            aria-label={`Eliminar ${item.title}`}
            onClick={() => onDelete(item)}
          >
            Eliminar
          </Button>
        </div>
      )}
    </div>
  );
}

export function Dashboard({
  data,
  accounts,
  recent,
  hidden,
  setHidden,
}: {
  data: Overview;
  accounts: Account[];
  recent: Movement[];
  hidden: boolean;
  setHidden: (hidden: boolean) => void;
}) {
  const unit = data.currency;
  const selected = accounts.filter((a) => a.currency === unit);
  const maxFlow = Math.max(
    ...data.flow.flatMap((item) => [Number(item.income), Number(item.expense)]),
    1,
  );
  const hasFlow = data.flow.some(
    (item) => Number(item.income) > 0 || Number(item.expense) > 0 || Number(item.investment) > 0,
  );
  return (
    <div className="page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Tu panorama financiero</div>
          <h1 className="page-title">Tus finanzas en Orden</h1>
          <div className="page-subtitle">
            Resumen de {monthLabel(data.month)} · {unit}. Las monedas se muestran por separado.
          </div>
        </div>
      </div>
      <section className="wealth-card">
        <div className="wealth-copy">
          <div className="wealth-label">
            Saldo en cuentas · {unit}
            <Button
              className="icon-button light"
              onClick={() => setHidden(!hidden)}
              aria-label={hidden ? "Mostrar saldo" : "Ocultar saldo"}
            >
              <Icon name={hidden ? "eyeOff" : "eye"} size={19} />
            </Button>
          </div>
          <div className={`wealth-value ${hidden ? "masked" : ""}`}>
            {hidden ? "••••••" : money(data.balance, unit)}
          </div>
          <div className="wealth-trend">
            <span>{selected.length} cuentas · Saldo actual, sin valoración de mercado</span>
          </div>
        </div>
        <div className="wealth-context">
          <Icon name="wallet" size={52} />
          <span>Cada movimiento cuenta.</span>
        </div>
      </section>
      <div className="stats-grid">
        <StatCard
          label="Ingresos del mes"
          amount={money(data.income, unit)}
          note="Dinero que entra a tus cuentas"
          icon="income"
        />
        <StatCard
          label="Gastos del mes"
          amount={money(data.expense, unit)}
          note="Dinero destinado a tus gastos"
          icon="expense"
        />
        <StatCard
          label="Aportes de inversión"
          amount={money(data.investment, unit)}
          note="Salidas destinadas a inversión"
          icon="investment"
        />
      </div>
      <AnnualBalances key={unit} currency={unit} balance={data.balance} hidden={hidden} />
      <div className="dashboard-grid">
        <section className="panel">
          <SectionHeader
            title="Gastos por categoría"
            subtitle={`${monthLabel(data.month)} · ${unit}`}
          />
          {data.categories.length === 0 ? (
            <EmptyState>Aún no hay gastos en este mes.</EmptyState>
          ) : (
            <div className="category-list">
              {data.categories.map((c, index) => (
                <div key={c.name} className="category-item">
                  <div>
                    <span className={`legend-dot c${(index % 5) + 1}`} />
                    <span>{c.name}</span>
                    <strong>{money(c.amount, unit)}</strong>
                  </div>
                  <div className="category-track">
                    <i
                      style={{
                        width: `${
                          Number(data.expense) ? (Number(c.amount) / Number(data.expense)) * 100 : 0
                        }%`,
                      }}
                    />
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
        <section className="panel">
          <SectionHeader title="Flujo mensual" subtitle={`Últimos seis meses · ${unit}`} />
          <div className="chart-key">
            <span>
              <i className="income-key" />
              Ingresos
            </span>
            <span>
              <i className="expense-key" />
              Gastos
            </span>
          </div>
          {!hasFlow ? (
            <EmptyState>Aún no hay movimientos para mostrar.</EmptyState>
          ) : (
            <>
              <div className="bar-chart" aria-hidden="true">
                {data.flow.map((m) => (
                  <div className="bar-group" key={m.month}>
                    <div className="bars">
                      <div
                        className="bar income-bar"
                        style={{
                          height: `${(Number(m.income) / maxFlow) * 100}%`,
                          minHeight: Number(m.income) ? "4px" : "0",
                          visibility: Number(m.income) ? "visible" : "hidden",
                        }}
                      />
                      <div
                        className="bar expense-bar"
                        style={{
                          height: `${(Number(m.expense) / maxFlow) * 100}%`,
                          minHeight: Number(m.expense) ? "4px" : "0",
                          visibility: Number(m.expense) ? "visible" : "hidden",
                        }}
                      />
                    </div>
                    <span>{monthLabel(m.month)}</span>
                  </div>
                ))}
              </div>
              <details className="flow-detail">
                <summary>Ver importes y aportes</summary>
                <table>
                  <caption className="sr-only">Flujo mensual en {unit}</caption>
                  <thead>
                    <tr>
                      <th>Mes</th>
                      <th>Ingresos</th>
                      <th>Gastos</th>
                      <th>Aportes</th>
                    </tr>
                  </thead>
                  <tbody>
                    {data.flow.map((m) => (
                      <tr key={m.month}>
                        <td>{monthLabel(m.month)}</td>
                        <td>{money(m.income, unit)}</td>
                        <td>{money(m.expense, unit)}</td>
                        <td>{money(m.investment, unit)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </details>
            </>
          )}
        </section>
      </div>
      <div className="dashboard-grid lower">
        <section className="panel">
          <SectionHeader
            title="Dónde está tu dinero"
            subtitle={`Saldo actual por cuenta · ${unit}`}
          />
          {selected.length === 0 ? (
            <EmptyState>Crea tu primera cuenta en la sección Cuentas.</EmptyState>
          ) : (
            <div className="account-list">
              {selected.map((a) => (
                <div className="account-row" key={a.id}>
                  <div className="account-icon sage">
                    <Icon name="wallet" size={18} />
                  </div>
                  <div className="account-copy">
                    <strong>{a.name}</strong>
                    <span>{a.type}</span>
                  </div>
                  <strong className="account-amount">{money(a.balance, unit)}</strong>
                </div>
              ))}
            </div>
          )}
        </section>
        <section className="panel">
          <SectionHeader
            title="Últimos movimientos"
            subtitle={`${monthLabel(data.month)} · ${unit}`}
          />
          <div className="movement-list compact">
            {recent.length === 0 ? (
              <EmptyState>Aún no hay movimientos en este mes.</EmptyState>
            ) : (
              recent.map((m) => <MovementRow key={m.id} item={m} />)
            )}
          </div>
        </section>
      </div>
    </div>
  );
}

export function Accounts({
  accounts,
  currency,
  onAdd,
  onEdit,
  onDelete,
}: {
  accounts: Account[];
  currency: Currency;
  onAdd: () => void;
  onEdit: (account: Account) => void;
  onDelete: (account: Account) => void;
}) {
  const selected = accounts.filter((a) => a.currency === currency);
  const positiveTotal = selected.reduce((sum, a) => sum + Math.max(0, Number(a.balance)), 0);
  return (
    <div className="page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Finanzas / Cuentas</div>
          <h1 className="page-title">Cuentas</h1>
          <div className="page-subtitle">
            Saldos actuales en {currency}. Cambia la moneda para ver las otras cuentas.
          </div>
        </div>
        <Button className="primary-button" onClick={onAdd}>
          <Icon name="plus" size={18} />
          Añadir cuenta
        </Button>
      </div>
      {selected.length === 0 ? (
        <section className="panel">
          <EmptyState>
            Aún no tienes cuentas en {currency}. Añade una para registrar tus movimientos.
          </EmptyState>
        </section>
      ) : (
        <>
          <div className="accounts-grid">
            {selected.map((a, index) => (
              <section className="account-card" key={a.id}>
                <div className="account-card-top">
                  <div className={`account-icon large ${index % 2 === 0 ? "sage" : "lavender"}`}>
                    <Icon name="wallet" />
                  </div>
                  <div className="row-actions">
                    <Button
                      className="text-button"
                      onClick={() => onEdit(a)}
                      aria-label={`Editar cuenta ${a.name}`}
                    >
                      Editar
                    </Button>
                    <Button
                      className="text-button danger"
                      onClick={() => onDelete(a)}
                      aria-label={`Eliminar cuenta ${a.name}`}
                    >
                      Eliminar
                    </Button>
                  </div>
                </div>
                <div className="account-card-type">{a.type}</div>
                <div className="account-card-name">{a.name}</div>
                <div className="account-card-bottom">
                  <strong>{money(a.balance, a.currency)}</strong>
                  <span>{a.currency}</span>
                </div>
                <div className="form-note">Inicial: {money(a.opening_balance, a.currency)}</div>
              </section>
            ))}
          </div>
          <section className="panel">
            <SectionHeader
              title="Distribución de fondos"
              subtitle="Porcentaje de los saldos positivos en esta moneda"
            />
            {positiveTotal === 0 ? (
              <EmptyState>No hay saldos positivos para mostrar.</EmptyState>
            ) : (
              <div className="category-list">
                {selected
                  .filter((a) => Number(a.balance) > 0)
                  .map((a) => (
                    <div className="category-item" key={a.id}>
                      <div>
                        <span>{a.name}</span>
                        <strong>{((Number(a.balance) / positiveTotal) * 100).toFixed(1)}%</strong>
                      </div>
                      <div className="category-track">
                        <i
                          style={{
                            width: `${(Number(a.balance) / positiveTotal) * 100}%`,
                          }}
                        />
                      </div>
                    </div>
                  ))}
              </div>
            )}
          </section>
        </>
      )}
    </div>
  );
}

export function Ledger({
  accounts,
  onAccountsChanged,
  page,
  data,
  overview,
  search,
  setSearch,
  category,
  setCategory,
  onAdd,
  onEdit,
  onDelete,
  onPage,
  hasAccounts,
  pendingOnly,
  setPendingOnly,
}: {
  page: Exclude<Page, "dashboard" | "cuentas" | "ajustes" | "calculadora">;
  accounts: Account[];
  onAccountsChanged: () => Promise<void>;
  data: MovementPage;
  overview: Overview;
  search: string;
  setSearch: (value: string) => void;
  category: string;
  setCategory: (value: string) => void;
  onAdd: () => void;
  onEdit: (m: Movement) => void;
  onDelete: (m: Movement) => void;
  onPage: (offset: number) => void;
  hasAccounts: boolean;
  pendingOnly: boolean;
  setPendingOnly: (value: boolean) => void;
}) {
  const config = pageConfig[page];
  const [categories, setCategories] = useState<{ id: number; name: string }[]>([]);
  const [categoriesLoading, setCategoriesLoading] = useState(true);
  const [categoriesError, setCategoriesError] = useState(false);
  const [categoryRetry, setCategoryRetry] = useState(0);
  useEffect(() => {
    if (page !== "gastos") return;
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
  }, [page, categoryRetry]);
  const total =
    config.kind === "income"
      ? overview.income
      : config.kind === "expense"
        ? overview.expense
        : overview.investment;
  return (
    <div className="page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Finanzas / {config.title}</div>
          <h1 className="page-title">{config.title}</h1>
          <div className="page-subtitle">
            {monthLabel(overview.month)} · {overview.currency}
            {config.kind === "investment" && " · Cuentas e historial de aportes, no rentabilidad de cartera"}
          </div>
        </div>
        <Button
          className="primary-button"
          onClick={onAdd}
          disabled={!hasAccounts}
          title={!hasAccounts ? "Primero crea una cuenta" : undefined}
        >
          <Icon name="plus" size={18} />
          {config.kind === "investment" ? "Añadir aporte" : config.add}
        </Button>
      </div>
      {page === "inversiones" && (
        <InvestmentOverview accounts={accounts} overview={overview} />
      )}
      {page === "inversiones" && (
        <InvestmentAccounts
          accounts={accounts}
          currency={overview.currency}
          onAccountsChanged={onAccountsChanged}
        />
      )}
      {page === "inversiones" && (
        <SectionHeader
          title="Historial de aportes"
          subtitle="Los aportes registrados descuentan saldo de su cuenta de origen; no son el saldo de tus cuentas de inversión."
        />
      )}
      <div className="stats-grid ledger-stats">
        <StatCard
          label="Total del mes"
          amount={money(total, overview.currency)}
          note="Incluye todos los movimientos de este tipo, sin el filtro de búsqueda"
          icon={config.kind}
        />
        <StatCard
          label="Resultados filtrados"
          amount={String(data.total)}
          note="Filtra por concepto y categoría"
          icon="home"
        />
        <StatCard
          label="Moneda seleccionada"
          amount={overview.currency}
          note="No se mezclan pesos y dólares"
          icon="wallet"
        />
      </div>
      <section className="panel ledger-panel">
        <div className="ledger-toolbar">
          {page === "gastos" && (
            <label className="pending-filter">
              <input
                type="checkbox"
                checked={pendingOnly}
                onChange={(e) => setPendingOnly(e.target.checked)}
              />{" "}
              Pendientes de completar · todos los meses
            </label>
          )}
          <label className="field search-field">
            <span>Buscar concepto</span>
            <input
              type="search"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              maxLength={150}
              placeholder="Buscar movimientos…"
            />
          </label>
          {page === "gastos" ? (
            <CategoryPicker
              mode="filter"
              value={category}
              categories={[...categories, { id: 0, name: "Sin categoría" }]}
              loading={categoriesLoading}
              error={categoriesError}
              onRetry={() => setCategoryRetry((value) => value + 1)}
              onChange={setCategory}
            />
          ) : (
            <label className="field search-field">
              <span>Categoría exacta</span>
              <input
                value={category}
                onChange={(e) => setCategory(e.target.value)}
                maxLength={100}
                placeholder="Todas las categorías"
              />
            </label>
          )}
        </div>
        {page === "gastos" && overview.categories.length > 0 && (
          <div
            className="ledger-category-shortcuts"
            role="group"
            aria-label="Filtros rápidos de categoría"
          >
            <span>Accesos rápidos</span>
            <Button
              className={`category-option ${!category ? "is-selected" : ""}`}
              aria-pressed={!category}
              onClick={() => setCategory("")}
            >
              Todos
            </Button>
            {overview.categories.slice(0, 5).map((item) => (
              <Button
                key={item.name}
                className={`category-option ${category === item.name ? "is-selected" : ""}`}
                aria-pressed={category === item.name}
                title={item.name}
                onClick={() => setCategory(category === item.name ? "" : item.name)}
              >
                {item.name.split(" > ").pop()}
              </Button>
            ))}
          </div>
        )}
        <div className="movement-list">
          {data.items.length === 0 ? (
            <EmptyState>
              {hasAccounts
                ? "No hay movimientos que coincidan con estos filtros."
                : "Crea una cuenta antes de registrar movimientos."}
            </EmptyState>
          ) : (
            data.items.map((m) => (
              <MovementRow key={m.id} item={m} onEdit={onEdit} onDelete={onDelete} />
            ))
          )}
        </div>
        <div className="ledger-footer">
          <span>
            {data.total === 0
              ? "0 resultados"
              : `${data.offset + 1}–${data.offset + data.items.length} de ${data.total}`}
          </span>
          <div className="row-actions">
            <Button
              className="secondary-button"
              onClick={() => onPage(Math.max(0, data.offset - data.limit))}
              disabled={data.offset === 0}
            >
              Anterior
            </Button>
            <Button
              className="secondary-button"
              onClick={() => onPage(data.offset + data.limit)}
              disabled={data.offset + data.items.length >= data.total}
            >
              Siguiente
            </Button>
          </div>
        </div>
      </section>
    </div>
  );
}

export default Dashboard;
