import type { Account, Overview } from "../lib/types";
import { monthLabel } from "../lib/types";
import { exactMoney, minor, percent, portfolio, ratio } from "../lib/metrics";
import { EmptyState, SectionHeader, StatCard } from "./ui";

export default function InvestmentOverview({
  accounts,
  overview,
}: {
  accounts: Account[];
  overview: Overview;
}) {
  const currency = overview.currency;
  const p = portfolio(accounts, currency);
  const largest = p.entries[0];
  const contributions = overview.flow.map((m) => ({
    month: m.month,
    value: minor(m.investment, currency),
  }));
  const max = contributions.reduce((n, m) => (m.value > n ? m.value : n), 0n);
  return (
    <section aria-label="Panorama de inversiones" className="investment-overview">
      <div className="stats-grid">
        <StatCard
          label="Total en inversiones"
          amount={exactMoney(p.total, currency)}
          note={`${p.entries.length} cuentas seleccionadas · saldo actual, no depende del mes`}
          icon="investment"
        />
        <StatCard
          label="De tu dinero total"
          amount={percent(p.share)}
          note={`Otras cuentas: ${exactMoney(p.totalMoney - p.total, currency)} · misma moneda`}
          icon="wallet"
        />
        <StatCard
          label="Mayor posición"
          amount={largest && largest.value > 0n ? largest.account.name : "Sin posición positiva"}
          note={
            largest && largest.value > 0n
              ? `${exactMoney(largest.value, currency)}${p.canDistribute ? ` · ${percent(ratio(largest.value, p.total))} de la cartera` : ""}`
              : "Selecciona tus cuentas de inversión más abajo"
          }
          icon="home"
        />
      </div>
      <div className="dashboard-grid">
        <section className="panel" aria-label="Distribución de inversiones">
          <SectionHeader
            title="Distribución de tu cartera"
            subtitle={`Saldo actual por cuenta · ${currency}`}
          />
          {!p.entries.length ? (
            <EmptyState>Añade cuentas existentes para ver tu distribución.</EmptyState>
          ) : (
            <div className="category-list">
              {p.entries.map(({ account, value }) => (
                <div className="category-item portfolio-position" key={account.id}>
                  <div>
                    <span>{account.name}</span>
                    <strong>{exactMoney(value, currency)}</strong>
                    <span>{p.canDistribute ? percent(ratio(value, p.total)) : "—"}</span>
                  </div>
                  {p.canDistribute && (
                    <div className="category-track">
                      <i style={{ width: `${Number(ratio(value, p.total) ?? 0n) / 100}%` }} />
                    </div>
                  )}
                </div>
              ))}
            </div>
          )}
          <p className="form-note">
            {p.entries.some((a) => a.value < 0n)
              ? "Hay saldos negativos: mostramos los importes netos, sin un gráfico porcentual engañoso. "
              : ""}
            El total ya está incluido en tu dinero del Resumen; no se suma otra vez. No es una
            cotización de mercado.
          </p>
          {p.share !== null && p.share > 10000n && (
            <p className="form-note">
              El peso supera 100% porque otras cuentas tienen saldo neto negativo.
            </p>
          )}
        </section>
        <section className="panel" aria-label="Evolución de aportes">
          <SectionHeader
            title="Aportes registrados"
            subtitle={`Seis meses hasta ${monthLabel(overview.month)} · ${currency}`}
          />
          {max === 0n ? (
            <EmptyState>No hay aportes registrados en este período.</EmptyState>
          ) : (
            <div className="contribution-list">
              {contributions.map((m) => (
                <div key={m.month}>
                  <div className="contribution-label">
                    <span>{monthLabel(m.month)}</span>
                    <strong>{exactMoney(m.value, currency)}</strong>
                  </div>
                  <div className="category-track">
                    <i style={{ width: `${Number(ratio(m.value, max) ?? 0n) / 100}%` }} />
                  </div>
                </div>
              ))}
            </div>
          )}
          <p className="form-note">
            Los aportes incluyen dinero externo y traslados desde otras cuentas. No miden ganancias
            ni rentabilidad.
          </p>
        </section>
      </div>
    </section>
  );
}
