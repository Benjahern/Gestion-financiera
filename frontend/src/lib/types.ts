export type Currency = "CLP" | "USD";
export type Page = "dashboard" | "gastos" | "ingresos" | "inversiones" | "cuentas" | "ajustes" | "calculadora";
export type Kind = "expense" | "income" | "investment";
export type Session = { email: string; csrf_token: string };
export type Account = {
  id: number;
  name: string;
  type: string;
  currency: Currency;
  opening_balance: string;
  balance: string;
  is_investment: boolean;
  show_in_summary: boolean;
};
export type Movement = {
  id: number;
  account_id: number;
  account: string;
  currency: Currency;
  title: string;
  category: string;
  date: string;
  amount: string;
  kind: Kind;
  pending: boolean;
  category_confirmed: boolean;
};
export type MovementPage = {
  items: Movement[];
  total: number;
  limit: number;
  offset: number;
};
export type Overview = {
  currency: Currency;
  month: string;
  balance: string;
  income: string;
  expense: string;
  investment: string;
  count: number;
  categories: { name: string; amount: string }[];
  flow: { month: string; income: string; expense: string; investment: string }[];
};
export const pageConfig = {
  gastos: { title: "Gastos", add: "Añadir gasto", kind: "expense" as Kind },
  ingresos: {
    title: "Ingresos",
    add: "Añadir ingreso",
    kind: "income" as Kind,
  },
  inversiones: {
    title: "Inversiones",
    add: "Añadir inversión",
    kind: "investment" as Kind,
  },
};
export function today() {
  const now = new Date();
  return `${now.getFullYear()}-${String(now.getMonth() + 1).padStart(2, "0")}-${String(now.getDate()).padStart(2, "0")}`;
}
export const money = (amount: string | number, currency: Currency) =>
  new Intl.NumberFormat("es-CL", {
    style: "currency",
    currency,
    currencyDisplay: "code",
    minimumFractionDigits: currency === "CLP" ? 0 : 2,
    maximumFractionDigits: currency === "CLP" ? 0 : 2,
  }).format(Number(amount));
export const dateLabel = (date: string) =>
  new Intl.DateTimeFormat("es-CL").format(new Date(`${date}T12:00:00`));
export const monthLabel = (month: string) =>
  new Intl.DateTimeFormat("es-CL", { month: "short", year: "numeric" }).format(
    new Date(`${month}-01T12:00:00`),
  );
