import { useCallback, useEffect, useRef, useState } from "react";
import Auth from "./components/Auth";
import Settings from "./components/Settings";
import Calculator from "./components/Calculator";
import { pageFromLocation, pagePaths } from "./lib/navigation";
import { Accounts, Dashboard, Ledger } from "./components/Finance";
import { AccountForm, MovementForm } from "./components/Forms";
import { Brand, Button, EmptyState, Icon, Modal, type IconName } from "./components/ui";
import { api, ApiError, setCSRF } from "./lib/api";
import {
  pageConfig,
  today,
  type Account,
  type Currency,
  type Kind,
  type Movement,
  type MovementPage,
  type Overview,
  type Page,
  type Session,
} from "./lib/types";

const nav: { id: Page; label: string; icon: IconName }[] = [
  { id: "dashboard", label: "Resumen", icon: "home" },
  { id: "gastos", label: "Gastos", icon: "expense" },
  { id: "ingresos", label: "Ingresos", icon: "income" },
  { id: "inversiones", label: "Inversiones", icon: "investment" },
  { id: "cuentas", label: "Cuentas", icon: "wallet" },
  { id: "calculadora", label: "Calculadora", icon: "calculator" },
  { id: "ajustes", label: "Conexiones", icon: "wallet" },
];
type Form =
  | { type: "account"; account?: Account }
  | {
      type: "movement";
      kind: Kind;
      movement?: Movement;
    };
type Removal =
  | { type: "account"; item: Account }
  | {
      type: "movement";
      item: Movement;
    };

function Workspace({ onLogout }: { onLogout: () => void }) {
  const [page, setPage] = useState<Page>(
    pageFromLocation(window.location.pathname, window.location.search) ?? "dashboard",
  );
  const [missingPage, setMissingPage] = useState(() => pageFromLocation(window.location.pathname, window.location.search) === null);
  const [currency, setCurrency] = useState<Currency>("CLP");
  const [month, setMonth] = useState(today().slice(0, 7));
  const [search, setSearch] = useState("");
  const [category, setCategory] = useState("");
  const [pendingOnly, setPendingOnly] = useState(false);
  const [offset, setOffset] = useState(0);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [overview, setOverview] = useState<Overview | null>(null);
  const [movements, setMovements] = useState<MovementPage>({
    items: [],
    total: 0,
    offset: 0,
    limit: 50,
  });
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const [hidden, setHidden] = useState(false);
  const [form, setForm] = useState<Form | null>(null);
  const [removal, setRemoval] = useState<Removal | null>(null);
  const [actionBusy, setActionBusy] = useState(false);
  const [actionError, setActionError] = useState("");
  const requestID = useRef(0);
  const accountsLoaded = useRef(false);
  const overviewKey = useRef("");
  const loadedPage = useRef<Page | "">("");
  const load = useCallback(
    async (signal?: AbortSignal) => {
      const id = ++requestID.current;
      setLoading(true);
      setError("");
      try {
        const query = new URLSearchParams({
          currency,
          month,
          limit: page === "dashboard" ? "5" : "50",
          offset: String(offset),
        });
        if (page === "gastos" && pendingOnly) {
          query.set("pending", "true");
          query.delete("month");
        }
        const ledgerPage = page === "gastos" || page === "ingresos" || page === "inversiones";
        if (ledgerPage)
          query.set("kind", pageConfig[page].kind);
        if (ledgerPage) {
          query.set("search", search.trim());
          query.set("category", category.trim());
        }
        const financialPage = page === "dashboard" || ledgerPage;
        const summaryKey = `${currency}:${month}`;
        const refresh = !signal || loadedPage.current !== page;
        const [a, o, m] = await Promise.all([
          !accountsLoaded.current || refresh ? api<Account[]>("/accounts", { signal }) : null,
          financialPage && (overviewKey.current !== summaryKey || refresh)
            ? api<Overview>(`/overview?${new URLSearchParams({ currency, month })}`, { signal })
            : null,
          financialPage ? api<MovementPage>(`/movements?${query}`, { signal }) : null,
        ]);
        if (id !== requestID.current || signal?.aborted) return;
        if (m && m.total > 0 && m.items.length === 0 && offset > 0) {
          setOffset(Math.max(0, offset - 50));
          return;
        }
        if (a) { setAccounts(a); accountsLoaded.current = true; }
        if (o) { setOverview(o); overviewKey.current = summaryKey; }
        if (m) setMovements(m);
        loadedPage.current = page;
        // A mutation on Settings/Accounts must invalidate financial summaries.
        if (!signal && !financialPage) overviewKey.current = "";
      } catch (err) {
        if (id !== requestID.current || signal?.aborted) return;
        setError(err instanceof Error ? err.message : "No se pudieron cargar tus datos.");
      } finally {
        if (id === requestID.current && !signal?.aborted) setLoading(false);
      }
    },
    [currency, month, page, offset, search, category, pendingOnly],
  );
  useEffect(() => {
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      void load(controller.signal);
    }, 200);
    return () => {
      controller.abort();
      window.clearTimeout(timer);
    };
  }, [load]);
  const navigate = (next: Page) => {
    if (window.location.pathname !== pagePaths[next] || window.location.search) window.history.pushState(null, "", pagePaths[next]);
    setMissingPage(false);
    setPage(next);
    setOffset(0);
    setSearch("");
    setCategory("");
    setPendingOnly(false);
    setForm(null);
    setRemoval(null);
  };
  useEffect(() => {
    const initial = pageFromLocation(window.location.pathname, window.location.search);
    if (initial) window.history.replaceState(null, "", pagePaths[initial] + window.location.search + window.location.hash);
    const backOrForward = () => {
      const next = pageFromLocation(window.location.pathname, window.location.search);
      setMissingPage(next === null);
      setPage(next ?? "dashboard");
      setOffset(0); setSearch(""); setCategory(""); setPendingOnly(false);
      setForm(null); setRemoval(null);
    };
    window.addEventListener("popstate", backOrForward);
    return () => window.removeEventListener("popstate", backOrForward);
  }, []);
  const follow = (event: React.MouseEvent<HTMLAnchorElement>, next: Page) => {
    if (event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) return;
    event.preventDefault();
    navigate(next);
  };
  const logout = async (all = false) => {
    setActionBusy(true);
    setActionError("");
    try {
      await api(all ? "/auth/logout-all" : "/auth/logout", { method: "POST" });
      onLogout();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "No se pudo cerrar la sesión.");
    } finally {
      setActionBusy(false);
    }
  };
  const remove = async () => {
    if (!removal) return;
    setActionBusy(true);
    setActionError("");
    try {
      await api(`/${removal.type === "account" ? "accounts" : "movements"}/${removal.item.id}`, {
        method: "DELETE",
      });
      setRemoval(null);
      await load();
    } catch (err) {
      setActionError(err instanceof Error ? err.message : "No se pudo eliminar.");
    } finally {
      setActionBusy(false);
    }
  };
  const requestRemoval = (target: Removal) => {
    setActionError("");
    setRemoval(target);
  };
  return (
    <div className="app-shell">
      <aside className="sidebar">
        <Brand />
        <nav className="sidebar-nav" aria-label="Navegación principal">
          {nav.map((item) => (
            <a
              key={item.id}
              className={`nav-item ${page === item.id ? "active" : ""}`}
              href={pagePaths[item.id]}
              aria-current={!missingPage && page === item.id ? "page" : undefined}
              onClick={event => follow(event, item.id)}
            >
              <Icon name={item.icon} />
              <span>{item.label}</span>
            </a>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="avatar">
            <Icon name="wallet" size={18} />
          </div>
          <div className="profile-copy">
            <strong>Acceso personal</strong>
            <span>Sesión protegida</span>
          </div>
        </div>
        <Button
          className="text-button logout-all"
          disabled={actionBusy}
          onClick={() => {
            if (window.confirm("¿Cerrar todas tus sesiones, incluidos tus otros dispositivos?"))
              void logout(true);
          }}
        >
          Cerrar todas las sesiones
        </Button>
      </aside>
      <nav className="mobile-nav" aria-label="Navegación móvil">
        {nav.map((item) => (
          <a
            key={item.id}
            className={`mobile-nav-item ${page === item.id ? "active" : ""}`}
            href={pagePaths[item.id]}
            aria-current={!missingPage && page === item.id ? "page" : undefined}
            onClick={event => follow(event, item.id)}
          >
            <Icon name={item.icon} size={19} />
            <span>{item.label}</span>
          </a>
        ))}
      </nav>
      <main className="main-content">
        <header className="topbar">
          <div className="mobile-brand">
            <Brand />
          </div>
          <div className="topbar-actions">
            {page !== "calculadora" && <label className="toolbar-label">
              <span>Mes</span>
              <input
                type="month"
                value={month}
                onChange={(e) => {
                  if (/^\d{4}-\d{2}$/.test(e.target.value)) {
                    setMonth(e.target.value);
                    setOffset(0);
                  }
                }}
                aria-label="Mes del resumen"
              />
            </label>}
            <label className="toolbar-label">
              <span>Moneda</span>
              <select
                value={currency}
                onChange={(e) => {
                  setCurrency(e.target.value as Currency);
                  setOffset(0);
                }}
                aria-label="Moneda"
              >
                <option value="CLP">CLP</option>
                <option value="USD">USD</option>
              </select>
            </label>
            <Button
              className="secondary-button"
              disabled={actionBusy}
              onClick={() => void logout()}
            >
              Salir
            </Button>
          </div>
        </header>
        {actionError && !removal && (
          <div className="page-error error-message" role="alert">
            {actionError}
          </div>
        )}
        {loading && overview && (
          <div className="loading-strip" role="status">
            Actualizando datos…
          </div>
        )}
        {missingPage ? (
          <div className="page"><h1 className="page-title">Página no encontrada</h1><p>Esta sección no existe.</p><Button className="secondary-button" onClick={() => navigate("dashboard")}>Ir al resumen</Button></div>
        ) : loading && !accountsLoaded.current ? (
          <EmptyState>
            <span role="status">Cargando tus finanzas…</span>
          </EmptyState>
        ) : error ? (
          <div className="page">
            <div className="error-message" role="alert">
              {error}
            </div>
            <Button className="secondary-button" onClick={() => void load()}>
              Reintentar
            </Button>
          </div>
        ) : (
          (page === "ajustes" ? (
            <Settings accounts={accounts} onData={load} />
          ) : page === "calculadora" ? (
            <Calculator key={currency} currency={currency} accounts={accounts} />
          ) : page === "dashboard" ? (
            overview && <Dashboard
              data={overview}
              accounts={accounts}
              recent={movements.items}
              hidden={hidden}
              setHidden={setHidden}
            />
          ) : page === "cuentas" ? (
            <Accounts
              accounts={accounts}
              onAccountsChanged={load}
              currency={currency}
              onAdd={() => setForm({ type: "account" })}
              onEdit={(account) => setForm({ type: "account", account })}
              onDelete={(item) => requestRemoval({ type: "account", item })}
            />
          ) : (
            overview && <Ledger
              accounts={accounts}
              onAccountsChanged={load}
              page={page}
              pendingOnly={pendingOnly}
              setPendingOnly={(value) => {
                setPendingOnly(value);
                setOffset(0);
              }}
              data={movements}
              overview={overview}
              search={search}
              setSearch={(value) => {
                setSearch(value);
                setOffset(0);
              }}
              category={category}
              setCategory={(value) => {
                setCategory(value);
                setOffset(0);
              }}
              onPage={setOffset}
              hasAccounts={accounts.some((a) => a.currency === currency)}
              onAdd={() => setForm({ type: "movement", kind: pageConfig[page].kind })}
              onEdit={(movement) => setForm({ type: "movement", kind: movement.kind, movement })}
              onDelete={(item) => requestRemoval({ type: "movement", item })}
            />
          ))
        )}
        <footer className="app-footer">
          Orden · CLP y USD separados ·{" "}
          <Button
            className="text-button mobile-logout-all"
            disabled={actionBusy}
            onClick={() => {
              if (window.confirm("¿Cerrar las sesiones de todos tus dispositivos?"))
                void logout(true);
            }}
          >
            Cerrar todas las sesiones
          </Button>
        </footer>
      </main>
      {form?.type === "account" && (
        <AccountForm
          account={form.account}
          currency={currency}
          onClose={() => setForm(null)}
          onSaved={load}
        />
      )}
      {form?.type === "movement" && (
        <MovementForm
          movement={form.movement}
          kind={form.kind}
          accounts={accounts}
          currency={currency}
          onClose={() => setForm(null)}
          onSaved={load}
        />
      )}
      {removal && (
        <Modal
          title={`Eliminar ${removal.type === "account" ? "cuenta" : "movimiento"}`}
          onClose={() => {
            if (!actionBusy) setRemoval(null);
          }}
        >
          <p className="form-note">
            ¿Eliminar «{removal.type === "account" ? removal.item.name : removal.item.title}
            »? Esta acción no se puede deshacer.
            {removal.type === "account" &&
              " Una cuenta con movimientos asociados no puede eliminarse."}
          </p>
          {actionError && (
            <div className="error-message" role="alert">
              {actionError}
            </div>
          )}
          <div className="modal-actions">
            <Button
              className="secondary-button"
              disabled={actionBusy}
              onClick={() => setRemoval(null)}
            >
              Cancelar
            </Button>
            <Button
              className="primary-button destructive-button"
              disabled={actionBusy}
              onClick={() => void remove()}
            >
              {actionBusy ? "Eliminando…" : "Eliminar"}
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

export default function App() {
  const [session, setSession] = useState<Session | null>(null);
  const [checking, setChecking] = useState(true);
  const [error, setError] = useState("");
  const [retry, setRetry] = useState(0);
  const logout = useCallback(() => {
    setCSRF("");
    setSession(null);
  }, []);
  useEffect(() => {
    const controller = new AbortController();
    setChecking(true);
    setError("");
    api<Session>("/auth/session", { signal: controller.signal })
      .then((value) => {
        if (!controller.signal.aborted) {
          setCSRF(value.csrf_token);
          setSession(value);
        }
      })
      .catch((err) => {
        if (!controller.signal.aborted && !(err instanceof ApiError && err.status === 401))
          setError(err instanceof Error ? err.message : "No se pudo verificar la sesión.");
      })
      .finally(() => {
        if (!controller.signal.aborted) setChecking(false);
      });
    window.addEventListener("orden:session-expired", logout);
    const hideAfterCacheRestore = (event: PageTransitionEvent) => {
      if (event.persisted) window.location.reload();
    };
    window.addEventListener("pageshow", hideAfterCacheRestore);
    return () => {
      controller.abort();
      window.removeEventListener("orden:session-expired", logout);
      window.removeEventListener("pageshow", hideAfterCacheRestore);
    };
  }, [retry, logout]);
  if (checking)
    return (
      <main className="auth-shell">
        <div className="auth-card">
          <Brand />
          <EmptyState>
            <span role="status">Verificando acceso…</span>
          </EmptyState>
        </div>
      </main>
    );
  if (error)
    return (
      <main className="auth-shell">
        <div className="auth-card">
          <Brand />
          <div className="error-message" role="alert">
            {error}
          </div>
          <Button className="primary-button" onClick={() => setRetry((value) => value + 1)}>
            Reintentar conexión
          </Button>
        </div>
      </main>
    );
  if (!session)
    return (
      <Auth
        onLogin={(value) => {
          setCSRF(value.csrf_token);
          setSession(value);
        }}
      />
    );
  return <Workspace onLogout={logout} />;
}
