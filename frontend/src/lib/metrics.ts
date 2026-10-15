import type { Account, Currency } from "./types";

// Read-only arithmetic: exact pesos/cents, including negative account balances.
export function minor(value: string, currency: Currency): bigint {
  if (!/^-?\d+(\.\d{1,2})?$/.test(value)) throw new Error("Importe inválido");
  const negative = value.startsWith("-");
  const [whole, fraction = ""] = value.replace(/^-/, "").split(".");
  if (currency === "CLP" && /[1-9]/.test(fraction)) throw new Error("CLP no admite fracciones");
  const n = currency === "CLP" ? BigInt(whole) : BigInt(whole) * 100n + BigInt((fraction + "00").slice(0, 2));
  return negative ? -n : n;
}
export function exactMoney(value: bigint, currency: Currency): string {
  const absolute = value < 0n ? -value : value;
  const scale = currency === "USD" ? 100n : 1n;
  const whole = new Intl.NumberFormat("es-CL", { maximumFractionDigits: 0 }).format(absolute / scale);
  return `${value < 0n ? "−" : ""}${currency}\u00a0${whole}${currency === "USD" ? `,${String(absolute % 100n).padStart(2, "0")}` : ""}`;
}
// Rounded hundredths of a percent. Zero/negative denominators aren't comparable.
export function ratio(numerator: bigint, denominator: bigint): bigint | null {
  if (denominator <= 0n) return null;
  const absolute = numerator < 0n ? -numerator : numerator;
  const rounded = (absolute * 10000n + denominator / 2n) / denominator;
  return numerator < 0n ? -rounded : rounded;
}
export function percent(value: bigint | null, signed = false): string {
  if (value === null) return "No comparable";
  const absolute = value < 0n ? -value : value;
  return `${value < 0n ? "−" : signed && value > 0n ? "+" : ""}${new Intl.NumberFormat("es-CL").format(absolute / 100n)},${String(absolute % 100n).padStart(2, "0")}%`;
}
export function annualChange(current: bigint, previous: bigint | undefined) {
  if (previous === undefined) return { difference: null, percentage: null };
  const difference = current - previous;
  return { difference, percentage: ratio(difference, previous) };
}
export function portfolio(accounts: Account[], currency: Currency) {
  const all = accounts.filter(a => a.currency === currency);
  const entries = all.filter(a => a.is_investment).map(account => ({ account, value: minor(account.balance, currency) }))
    .sort((a,b) => a.value > b.value ? -1 : a.value < b.value ? 1 : a.account.id - b.account.id);
  const total = entries.reduce((sum,a) => sum + a.value, 0n);
  const totalMoney = all.reduce((sum,a) => sum + minor(a.balance, currency), 0n);
  const canDistribute = total > 0n && entries.every(a => a.value >= 0n);
  return { entries, total, totalMoney, share: total >= 0n ? ratio(total, totalMoney) : null, canDistribute };
}
