import type { Currency } from "./types";

export type CalculatorSettings = {
  currency: Currency;
  reference_income: string;
  general: string;
  wants: string;
  investment: string;
  index_fund: string;
  reserve: string;
  emergency_goal: string;
  emergency_balance: string;
  own_home_percent: string;
  rental_percent: string;
  travel_percent: string;
};
export type PlanningField = Exclude<keyof CalculatorSettings, "currency">;
export const emptySettings = (currency: Currency): CalculatorSettings => ({
  currency, reference_income: "", general: "", wants: "", investment: "",
  index_fund: "", reserve: "", emergency_goal: "", emergency_balance: "",
  own_home_percent: "", rental_percent: "", travel_percent: "",
});

export function units(input: string, currency: Currency): bigint {
  const value = input.trim().replace(",", ".");
  if (!/^\d{1,12}(?:\.\d{1,2})?$/.test(value)) throw new Error("Usa importes positivos sin separadores de miles; hasta dos decimales en USD.");
  const [whole, fraction = ""] = value.split(".");
  if (currency === "CLP") {
    if (fraction.replace(/0/g, "")) throw new Error("CLP no admite fracciones de peso.");
    return BigInt(whole);
  }
  return BigInt(whole) * 100n + BigInt((fraction + "00").slice(0, 2));
}
export function amount(value: bigint, currency: Currency): string {
  return currency === "CLP" ? value.toString() : `${value / 100n}.${(value % 100n).toString().padStart(2, "0")}`;
}

export function validateSettings(c: CalculatorSettings) {
  const values = {
    reference_income: units(c.reference_income, c.currency), general: units(c.general, c.currency),
    wants: units(c.wants, c.currency), investment: units(c.investment, c.currency),
    index_fund: units(c.index_fund, c.currency), reserve: units(c.reserve, c.currency),
    emergency_goal: units(c.emergency_goal, c.currency), emergency_balance: units(c.emergency_balance, c.currency),
  };
  if (values.reference_income === 0n || values.emergency_goal === 0n) throw new Error("El sueldo de referencia y la meta deben ser mayores que cero.");
  if (values.general + values.wants + values.investment !== values.reference_income) throw new Error("Gastos generales, caprichos e inversión deben sumar el sueldo de referencia.");
  if (values.index_fund + values.reserve !== values.investment) throw new Error("Fondo indexado y reserva deben sumar la inversión de referencia.");
  const weights = [c.own_home_percent, c.rental_percent, c.travel_percent].map(p => units(p, "USD"));
  if (weights.some(p => p > 10000n) || weights.reduce((a, b) => a + b, 0n) !== 10000n) throw new Error("Los porcentajes de casa propia, casa de inversión y viaje deben sumar 100%.");
  return { ...values, weights };
}

// Largest remainders: every peso/cent is assigned exactly once, including ties.
export function distribute(total: bigint, weights: bigint[]): bigint[] {
  const sum = weights.reduce((a, b) => a + b, 0n);
  if (!sum) return weights.map(() => 0n);
  const parts = weights.map(w => total * w / sum);
  const order = weights.map((w, i) => ({ i, remainder: total * w % sum }))
    .sort((a, b) => a.remainder === b.remainder ? a.i - b.i : a.remainder > b.remainder ? -1 : 1);
  let left = total - parts.reduce((a, b) => a + b, 0n);
  for (const { i } of order) { if (!left) break; parts[i] += 1n; left -= 1n; }
  return parts;
}
export function calculate(c: CalculatorSettings, input: string) {
  const v = validateSettings(c);
  const income = units(input, c.currency);
  const [general, wants, investment] = distribute(income, [v.general, v.wants, v.investment]);
  const [index, reserve] = distribute(investment, [v.index_fund, v.reserve]);
  const complete = v.emergency_balance >= v.emergency_goal;
  const [ownHome, rental, travel] = complete ? distribute(reserve, v.weights) : [0n, 0n, 0n];
  return { income, general, wants, investment, index, emergency: complete ? 0n : reserve,
    ownHome, rental, travel, complete, progress: Number(v.emergency_balance * 10000n / v.emergency_goal) / 100 };
}
