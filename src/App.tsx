import { createElement, useMemo, useState, type ReactNode } from "react";

type Page = "dashboard" | "gastos" | "ingresos" | "inversiones" | "cuentas";
type IconName =
  | "home"
  | "expense"
  | "income"
  | "investment"
  | "wallet"
  | "plus"
  | "eye"
  | "eyeOff"
  | "search"
  | "calendar"
  | "arrowUp"
  | "arrowDown"
  | "chevron"
  | "close"
  | "check"
  | "more";

type Movement = {
  id: number;
  title: string;
  category: string;
  date: string;
  amount: number;
  kind: "expense" | "income" | "investment";
  account: string;
};

const navItems: { id: Page; label: string; icon: IconName }[] = [
  { id: "dashboard", label: "Resumen", icon: "home" },
  { id: "gastos", label: "Gastos", icon: "expense" },
  { id: "ingresos", label: "Ingresos", icon: "income" },
  { id: "inversiones", label: "Inversiones", icon: "investment" },
  { id: "cuentas", label: "Cuentas", icon: "wallet" },
];

const initialMovements: Movement[] = [
  { id: 1, title: "Alquiler", category: "Vivienda", date: "18 Jun, 2025", amount: 820, kind: "expense", account: "Banco Aurora" },
  { id: 2, title: "Supermercado Verde", category: "Alimentación", date: "16 Jun, 2025", amount: 94.6, kind: "expense", account: "Tarjeta Nébula" },
  { id: 3, title: "Nómina mensual", category: "Sueldo", date: "15 Jun, 2025", amount: 2850, kind: "income", account: "Banco Aurora" },
  { id: 4, title: "ETF Global", category: "Fondos", date: "12 Jun, 2025", amount: 300, kind: "investment", account: "Faro Invest" },
  { id: 5, title: "Cena con amigos", category: "Ocio", date: "10 Jun, 2025", amount: 48.2, kind: "expense", account: "Tarjeta Nébula" },
  { id: 6, title: "Proyecto freelance", category: "Extra", date: "08 Jun, 2025", amount: 420, kind: "income", account: "Banco Aurora" },
  { id: 7, title: "Transporte mensual", category: "Transporte", date: "03 Jun, 2025", amount: 67.5, kind: "expense", account: "Efectivo" },
  { id: 8, title: "Acciones tecnológicas", category: "Acciones", date: "01 Jun, 2025", amount: 180, kind: "investment", account: "Faro Invest" },
];

const accounts = [
  { name: "Banco Aurora", type: "Cuenta principal", amount: 7280.4, color: "sage", digits: "1842" },
  { name: "Faro Invest", type: "Cartera de inversión", amount: 8420.7, color: "lavender", digits: "9041" },
  { name: "Tarjeta Nébula", type: "Crédito disponible", amount: 1840, color: "blue", digits: "4427" },
  { name: "Efectivo", type: "Dinero físico", amount: 380, color: "sand", digits: "" },
];

const currency = (value: number, decimals = 0) =>
  new Intl.NumberFormat("es-ES", {
    style: "currency",
    currency: "EUR",
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  }).format(value);

const iconPaths: Record<IconName, ReactNode> = {
  home: <><path d="M3 10.8 12 3l9 7.8" /><path d="M5.5 9.5V21h13V9.5M9 21v-7h6v7" /></>,
  expense: <><path d="M4 7h16v12H4z" /><path d="M4 10h16M8 15h3" /></>,
  income: <><path d="M12 3v13M7 11l5 5 5-5" /><path d="M5 21h14" /></>,
  investment: <><path d="M4 20V10M10 20V4M16 20v-7M22 20H2" /><path d="m4 7 6-4 6 7 5-5" /></>,
  wallet: <><path d="M3 6.5A2.5 2.5 0 0 1 5.5 4H19v16H5.5A2.5 2.5 0 0 1 3 17.5z" /><path d="M3 7h16M15 12h7v5h-7a2.5 2.5 0 0 1 0-5Z" /></>,
  plus: <><path d="M12 5v14M5 12h14" /></>,
  eye: <><path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z" /><circle cx="12" cy="12" r="2.5" /></>,
  eyeOff: <><path d="m3 3 18 18M10.6 6.2A9.7 9.7 0 0 1 12 6c6 0 9.5 6 9.5 6a15 15 0 0 1-2.2 2.8M6.3 6.3A15 15 0 0 0 2.5 12s3.5 6 9.5 6c1.5 0 2.8-.4 3.9-.9" /></>,
  search: <><circle cx="10.5" cy="10.5" r="6.5" /><path d="m16 16 5 5" /></>,
  calendar: <><rect x="3" y="5" width="18" height="16" rx="2" /><path d="M8 3v4M16 3v4M3 10h18" /></>,
  arrowUp: <><path d="m7 11 5-5 5 5M12 18V6" /></>,
  arrowDown: <><path d="m7 13 5 5 5-5M12 6v12" /></>,
  chevron: <path d="m9 6 6 6-6 6" />,
  close: <><path d="m6 6 12 12M18 6 6 18" /></>,
  check: <path d="m5 12 4 4L19 6" />,
  more: <><circle cx="5" cy="12" r="1" /><circle cx="12" cy="12" r="1" /><circle cx="19" cy="12" r="1" /></>,
};

function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return (
    <svg
      aria-hidden="true"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {iconPaths[name]}
    </svg>
  );
}

function Button({
  children,
  className = "",
  onClick,
  ariaLabel,
  type = "button",
}: {
  children: ReactNode;
  className?: string;
  onClick?: () => void;
  ariaLabel?: string;
  type?: "button" | "submit";
}) {
  return createElement("button", { className, onClick, "aria-label": ariaLabel, type }, children);
}

function Input(props: Record<string, unknown>) {
  return createElement("input", props);
}

function Select({
  value,
  onChange,
  options,
  className = "",
  ariaLabel,
}: {
  value: string;
  onChange: (value: string) => void;
  options: string[];
  className?: string;
  ariaLabel?: string;
}) {
  return createElement(
    "select",
    { value, onChange: (event: React.ChangeEvent<HTMLSelectElement>) => onChange(event.target.value), className, "aria-label": ariaLabel },
    options.map((option) => createElement("option", { key: option, value: option }, option)),
  );
}

function Brand() {
  return (
    <div className="brand">
      <div className="brand-mark"><span /><span /><span /></div>
      <div className="brand-name">Luma</div>
    </div>
  );
}

function Sidebar({ page, setPage }: { page: Page; setPage: (page: Page) => void }) {
  return (
    <>
      <aside className="sidebar">
        <Brand />
        <nav className="sidebar-nav" aria-label="Navegación principal">
          {navItems.map((item) => (
            <Button
              key={item.id}
              className={`nav-item ${page === item.id ? "active" : ""}`}
              onClick={() => setPage(item.id)}
            >
              <Icon name={item.icon} />
              <span>{item.label}</span>
            </Button>
          ))}
        </nav>
        <div className="sidebar-footer">
          <div className="avatar">MC</div>
          <div className="profile-copy">
            <strong>Martina Costa</strong>
            <span>Plan personal</span>
          </div>
          <Icon name="more" />
        </div>
      </aside>
      <nav className="mobile-nav" aria-label="Navegación principal móvil">
        {navItems.map((item) => (
          <Button
            key={item.id}
            className={`mobile-nav-item ${page === item.id ? "active" : ""}`}
            onClick={() => setPage(item.id)}
            ariaLabel={item.label}
          >
            <Icon name={item.icon} size={19} />
            <span>{item.id === "dashboard" ? "Inicio" : item.label}</span>
          </Button>
        ))}
      </nav>
    </>
  );
}

function Topbar() {
  return (
    <div className="topbar">
      <div className="mobile-brand"><Brand /></div>
      <div className="topbar-actions">
        <Button className="period-button">
          <Icon name="calendar" size={17} />
          <span>Junio 2025</span>
          <span className="tiny-chevron">⌄</span>
        </Button>
        <div className="status-pill"><span /> Todo al día</div>
        <div className="mobile-avatar">MC</div>
      </div>
    </div>
  );
}

function StatCard({
  label,
  amount,
  note,
  trend,
  icon,
}: {
  label: string;
  amount: string;
  note: string;
  trend?: "up" | "down";
  icon: IconName;
}) {
  return (
    <div className="stat-card">
      <div className="stat-head">
        <span>{label}</span>
        <div className="icon-tile"><Icon name={icon} size={18} /></div>
      </div>
      <div className="stat-value">{amount}</div>
      <div className={`stat-note ${trend || ""}`}>
        {trend && <Icon name={trend === "up" ? "arrowUp" : "arrowDown"} size={14} />}
        {note}
      </div>
    </div>
  );
}

function Dashboard({ hidden, setHidden, movements }: { hidden: boolean; setHidden: (value: boolean) => void; movements: Movement[] }) {
  const spent = movements.filter((item) => item.kind === "expense").reduce((sum, item) => sum + item.amount, 0);
  const invested = movements.filter((item) => item.kind === "investment").reduce((sum, item) => sum + item.amount, 0);
  return (
    <div className="page">
      <div className="page-intro dashboard-intro">
        <div>
          <div className="eyebrow">Tu panorama financiero</div>
          <div className="page-title">Buenos días, Martina</div>
          <div className="page-subtitle">Aquí tienes una mirada clara a tu dinero este mes.</div>
        </div>
      </div>

      <section className="wealth-card">
        <div className="wealth-copy">
          <div className="wealth-label">
            Patrimonio total
            <Button className="icon-button light" onClick={() => setHidden(!hidden)} ariaLabel={hidden ? "Mostrar patrimonio" : "Ocultar patrimonio"}>
              <Icon name={hidden ? "eyeOff" : "eye"} size={19} />
            </Button>
          </div>
          <div className={`wealth-value ${hidden ? "masked" : ""}`}>{hidden ? "••••••" : currency(17921.1, 2)}</div>
          <div className="wealth-trend"><Icon name="arrowUp" size={14} /> 6,8% <span>desde el mes pasado</span></div>
        </div>
        <div className="wealth-visual" aria-label="Evolución positiva del patrimonio">
          <svg viewBox="0 0 420 130" preserveAspectRatio="none">
            <defs>
              <linearGradient id="area" x1="0" x2="0" y1="0" y2="1">
                <stop offset="0%" stopColor="currentColor" stopOpacity=".28" />
                <stop offset="100%" stopColor="currentColor" stopOpacity="0" />
              </linearGradient>
            </defs>
            <path className="area" d="M0 108 C45 91 68 102 103 84 S160 90 195 67 250 76 284 49 351 58 420 17 L420 130 L0 130Z" />
            <path className="line" d="M0 108 C45 91 68 102 103 84 S160 90 195 67 250 76 284 49 351 58 420 17" />
            <circle cx="420" cy="17" r="4" />
          </svg>
        </div>
      </section>

      <div className="stats-grid">
        <StatCard label="Ingresos" amount={currency(3270)} note="+ 12% frente a mayo" trend="up" icon="income" />
        <StatCard label="Gastos" amount={currency(spent, 2)} note="43% del presupuesto" trend="down" icon="expense" />
        <StatCard label="Inversiones" amount={currency(invested)} note="15% de tus ingresos" trend="up" icon="investment" />
      </div>

      <div className="dashboard-grid">
        <ExpenseBreakdown />
        <MonthlyFlow />
      </div>

      <div className="dashboard-grid lower">
        <AccountsPreview />
        <RecentMovements movements={movements.slice(0, 4)} />
      </div>
    </div>
  );
}

function SectionHeader({ title, subtitle, action }: { title: string; subtitle?: string; action?: ReactNode }) {
  return (
    <div className="section-header">
      <div>
        <div className="section-title">{title}</div>
        {subtitle && <div className="section-subtitle">{subtitle}</div>}
      </div>
      {action}
    </div>
  );
}

function ExpenseBreakdown() {
  const categories = [
    { name: "Vivienda", value: 820, percent: 57, color: "c1" },
    { name: "Alimentación", value: 244, percent: 17, color: "c2" },
    { name: "Ocio", value: 189, percent: 13, color: "c3" },
    { name: "Transporte", value: 126, percent: 9, color: "c4" },
    { name: "Otros", value: 58, percent: 4, color: "c5" },
  ];
  return (
    <section className="panel">
      <SectionHeader title="Gastos por categoría" subtitle="Distribución de junio" action={<Button className="text-button">Ver detalle <Icon name="chevron" size={14} /></Button>} />
      <div className="expense-content">
        <div className="donut-wrap">
          <div className="donut">
            <div className="donut-center"><strong>{currency(1437)}</strong><span>Total gastado</span></div>
          </div>
        </div>
        <div className="legend">
          {categories.map((category) => (
            <div className="legend-row" key={category.name}>
              <span className={`legend-dot ${category.color}`} />
              <span className="legend-name">{category.name}</span>
              <strong>{category.percent}%</strong>
              <span>{currency(category.value)}</span>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}

function MonthlyFlow() {
  const data = [
    { month: "Ene", income: 61, expense: 39 },
    { month: "Feb", income: 74, expense: 45 },
    { month: "Mar", income: 68, expense: 51 },
    { month: "Abr", income: 82, expense: 43 },
    { month: "May", income: 71, expense: 55 },
    { month: "Jun", income: 92, expense: 41 },
  ];
  return (
    <section className="panel">
      <SectionHeader title="Flujo mensual" subtitle="Ingresos frente a gastos" />
      <div className="chart-key"><span><i className="income-key" />Ingresos</span><span><i className="expense-key" />Gastos</span></div>
      <div className="bar-chart">
        {data.map((item) => (
          <div className="bar-group" key={item.month}>
            <div className="bars">
              <div className="bar income-bar" style={{ height: `${item.income}%` }} />
              <div className="bar expense-bar" style={{ height: `${item.expense}%` }} />
            </div>
            <span>{item.month}</span>
          </div>
        ))}
      </div>
    </section>
  );
}

function AccountsPreview() {
  return (
    <section className="panel">
      <SectionHeader title="Dónde está tu dinero" subtitle="Saldo disponible por cuenta" />
      <div className="account-list">
        {accounts.slice(0, 3).map((account) => (
          <div className="account-row" key={account.name}>
            <div className={`account-icon ${account.color}`}><Icon name={account.name === "Faro Invest" ? "investment" : "wallet"} size={18} /></div>
            <div className="account-copy"><strong>{account.name}</strong><span>•• {account.digits}</span></div>
            <strong className="account-amount">{currency(account.amount, 2)}</strong>
          </div>
        ))}
      </div>
    </section>
  );
}

function RecentMovements({ movements }: { movements: Movement[] }) {
  return (
    <section className="panel">
      <SectionHeader title="Últimos movimientos" subtitle="Actividad reciente" />
      <div className="movement-list compact">
        {movements.map((item) => <MovementRow key={item.id} item={item} />)}
      </div>
    </section>
  );
}

function MovementRow({ item }: { item: Movement }) {
  const icon = item.kind === "expense" ? "expense" : item.kind === "income" ? "income" : "investment";
  return (
    <div className="movement-row">
      <div className={`movement-icon ${item.kind}`}><Icon name={icon} size={18} /></div>
      <div className="movement-primary"><strong>{item.title}</strong><span>{item.category} · {item.account}</span></div>
      <span className="movement-date">{item.date}</span>
      <strong className={`movement-amount ${item.kind}`}>
        {item.kind === "income" ? "+" : "−"}{currency(item.amount, 2)}
      </strong>
      <Button className="row-menu" ariaLabel={`Más opciones para ${item.title}`}><Icon name="more" size={18} /></Button>
    </div>
  );
}

const pageConfig = {
  gastos: {
    title: "Gastos",
    subtitle: "Entiende tus hábitos y mantén tu presupuesto bajo control.",
    add: "Añadir gasto",
    kind: "expense" as const,
    categories: ["Alimentación", "Vivienda", "Transporte", "Ocio", "Salud", "Otros"],
  },
  ingresos: {
    title: "Ingresos",
    subtitle: "Reúne todas tus fuentes de ingreso en un solo lugar.",
    add: "Añadir ingreso",
    kind: "income" as const,
    categories: ["Sueldo", "Extra", "Freelance", "Rentas", "Otros"],
  },
  inversiones: {
    title: "Inversiones",
    subtitle: "Sigue el crecimiento de tu dinero a largo plazo.",
    add: "Añadir inversión",
    kind: "investment" as const,
    categories: ["Fondos", "Acciones", "Cripto", "Ahorro", "Otros"],
  },
};

function LedgerPage({
  page,
  movements,
  onAdd,
}: {
  page: "gastos" | "ingresos" | "inversiones";
  movements: Movement[];
  onAdd: () => void;
}) {
  const config = pageConfig[page];
  const filtered = movements.filter((item) => item.kind === config.kind);
  const total = filtered.reduce((sum, item) => sum + item.amount, 0);
  const average = filtered.length ? total / filtered.length : 0;
  return (
    <div className="page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Finanzas / {config.title}</div>
          <div className="page-title">{config.title}</div>
          <div className="page-subtitle">{config.subtitle}</div>
        </div>
        <Button className="primary-button" onClick={onAdd}><Icon name="plus" size={18} />{config.add}</Button>
      </div>
      <div className="stats-grid ledger-stats">
        <StatCard
          label={`Total en junio`}
          amount={currency(total, 2)}
          note={page === "gastos" ? "8% menos que en mayo" : page === "ingresos" ? "12% más que en mayo" : "Rentabilidad anual +6,4%"}
          trend={page === "gastos" ? "down" : "up"}
          icon={page === "gastos" ? "expense" : page === "ingresos" ? "income" : "investment"}
        />
        <StatCard label="Promedio por movimiento" amount={currency(average, 2)} note={`${filtered.length} movimientos registrados`} icon="calendar" />
        <StatCard
          label={page === "gastos" ? "Presupuesto restante" : page === "ingresos" ? "Meta mensual" : "Valor de cartera"}
          amount={page === "gastos" ? currency(963) : page === "ingresos" ? "82%" : currency(8420.7, 2)}
          note={page === "gastos" ? "de 2.400 € disponibles" : page === "ingresos" ? "3.270 € de 4.000 €" : "Aportado 7.950 €"}
          icon="wallet"
        />
      </div>
      <section className="panel ledger-panel">
        <div className="ledger-toolbar">
          <div className="search-box"><Icon name="search" size={17} /><Input type="search" placeholder={`Buscar en ${config.title.toLowerCase()}…`} aria-label={`Buscar en ${config.title.toLowerCase()}`} /></div>
          <div className="filter-group">
            <Select value="Junio 2025" onChange={() => undefined} options={["Junio 2025", "Mayo 2025", "Abril 2025"]} className="filter-select" ariaLabel="Seleccionar mes" />
            <Select value="Todo el mes" onChange={() => undefined} options={["Todo el mes", "Esta semana", "Semana anterior"]} className="filter-select" ariaLabel="Seleccionar periodo" />
          </div>
        </div>
        <div className="ledger-heading">
          <span>Movimiento</span><span>Fecha</span><span>Importe</span><span />
        </div>
        <div className="movement-list">
          {filtered.map((item) => <MovementRow key={item.id} item={item} />)}
        </div>
        <div className="ledger-footer"><span>Mostrando {filtered.length} movimientos</span><span>Total: <strong>{currency(total, 2)}</strong></span></div>
      </section>
    </div>
  );
}

function AccountsPage() {
  return (
    <div className="page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Finanzas / Cuentas</div>
          <div className="page-title">Cuentas</div>
          <div className="page-subtitle">Todo tu dinero, organizado sin perderlo de vista.</div>
        </div>
        <Button className="primary-button"><Icon name="plus" size={18} />Añadir cuenta</Button>
      </div>
      <section className="account-hero">
        <div><span>Saldo total entre cuentas</span><strong>{currency(17921.1, 2)}</strong></div>
        <div className="account-hero-detail"><span>4 cuentas conectadas</span><span className="sync-dot" />Actualizado ahora</div>
      </section>
      <div className="accounts-grid">
        {accounts.map((account) => (
          <section className="account-card" key={account.name}>
            <div className="account-card-top">
              <div className={`account-icon large ${account.color}`}><Icon name={account.name === "Faro Invest" ? "investment" : "wallet"} /></div>
              <Button className="row-menu" ariaLabel={`Opciones de ${account.name}`}><Icon name="more" /></Button>
            </div>
            <div className="account-card-type">{account.type}</div>
            <div className="account-card-name">{account.name}</div>
            <div className="account-card-bottom"><strong>{currency(account.amount, 2)}</strong><span>{account.digits ? `•• ${account.digits}` : "Disponible"}</span></div>
          </section>
        ))}
      </div>
      <section className="panel">
        <SectionHeader title="Distribución de fondos" subtitle="Porcentaje de tu patrimonio en cada cuenta" />
        <div className="distribution-bar"><i /><i /><i /><i /></div>
        <div className="distribution-labels">
          {accounts.map((account, index) => (
            <span key={account.name}><i className={`dist-${index + 1}`} />{account.name}<strong>{Math.round((account.amount / 17921.1) * 100)}%</strong></span>
          ))}
        </div>
      </section>
    </div>
  );
}

function AddModal({
  page,
  onClose,
  onSave,
}: {
  page: "gastos" | "ingresos" | "inversiones";
  onClose: () => void;
  onSave: (movement: Movement) => void;
}) {
  const config = pageConfig[page];
  const [title, setTitle] = useState("");
  const [amount, setAmount] = useState("");
  const [category, setCategory] = useState(config.categories[0]);
  const [account, setAccount] = useState(accounts[0].name);
  const submit = () => {
    const parsed = Number(amount.replace(",", "."));
    if (!title.trim() || !parsed) return;
    onSave({ id: Date.now(), title: title.trim(), category, amount: parsed, account, kind: config.kind, date: "Hoy, 2025" });
  };
  return (
    <div className="modal-backdrop" role="presentation" onMouseDown={onClose}>
      <div className="modal" role="dialog" aria-modal="true" aria-labelledby="modal-title" onMouseDown={(event) => event.stopPropagation()}>
        <div className="modal-header">
          <div><div className="modal-title" id="modal-title">{config.add}</div><div className="modal-subtitle">Completa los datos del movimiento.</div></div>
          <Button className="icon-button" onClick={onClose} ariaLabel="Cerrar"><Icon name="close" /></Button>
        </div>
        <div className="form-grid">
          <label className="field full"><span>Concepto</span><Input value={title} onChange={(event: React.ChangeEvent<HTMLInputElement>) => setTitle(event.target.value)} placeholder={page === "gastos" ? "Ej. Compra semanal" : "Ej. Nómina mensual"} autoFocus /></label>
          <label className="field"><span>Importe</span><div className="amount-input"><Input inputMode="decimal" value={amount} onChange={(event: React.ChangeEvent<HTMLInputElement>) => setAmount(event.target.value)} placeholder="0,00" /><strong>EUR</strong></div></label>
          <label className="field"><span>Fecha</span><Input type="date" defaultValue="2025-06-20" /></label>
          <label className="field"><span>Categoría</span><Select value={category} onChange={setCategory} options={config.categories} /></label>
          <label className="field"><span>Cuenta</span><Select value={account} onChange={setAccount} options={accounts.map((item) => item.name)} /></label>
        </div>
        <div className="modal-actions">
          <Button className="secondary-button" onClick={onClose}>Cancelar</Button>
          <Button className="primary-button" onClick={submit}><Icon name="check" size={17} />Guardar movimiento</Button>
        </div>
      </div>
    </div>
  );
}

export default function App() {
  const [page, setPage] = useState<Page>("dashboard");
  const [hidden, setHidden] = useState(false);
  const [movements, setMovements] = useState(initialMovements);
  const [modal, setModal] = useState<"gastos" | "ingresos" | "inversiones" | null>(null);
  const content = useMemo(() => {
    if (page === "dashboard") return <Dashboard hidden={hidden} setHidden={setHidden} movements={movements} />;
    if (page === "cuentas") return <AccountsPage />;
    return <LedgerPage page={page} movements={movements} onAdd={() => setModal(page)} />;
  }, [page, hidden, movements]);
  return (
    <div className="app-shell">
      <Sidebar page={page} setPage={setPage} />
      <main className="main-content">
        <Topbar />
        {content}
      </main>
      {modal && (
        <AddModal
          page={modal}
          onClose={() => setModal(null)}
          onSave={(movement) => {
            setMovements((items) => [movement, ...items]);
            setModal(null);
          }}
        />
      )}
    </div>
  );
}
