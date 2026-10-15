import test from "node:test";
import assert from "node:assert/strict";
import { amount, calculate, distribute, units, validateSettings } from "../src/lib/calculator.ts";
import { pageFromLocation, pagePaths } from "../src/lib/navigation.ts";

// Synthetic fixtures, never imported from the personal workbook.
const fixture = { currency: "CLP", reference_income: "1000", general: "500", wants: "100", investment: "400", index_fund: "250", reserve: "150", emergency_goal: "100", emergency_balance: "99", own_home_percent: "50", rental_percent: "30", travel_percent: "20" };
test("Excel flow: emergency first, then goals without adding money", () => {
  const before = calculate(fixture, "1000");
  assert.equal(before.emergency, 150n); // matches Excel: no partial switch in same contribution
  assert.equal(before.ownHome, 0n);
  const after = calculate({ ...fixture, emergency_balance: "100" }, "1000");
  assert.deepEqual([after.emergency, after.ownHome, after.rental, after.travel], [0n, 75n, 45n, 30n]);
  for (const r of [before, after]) assert.equal(r.general + r.wants + r.index + r.emergency + r.ownHome + r.rental + r.travel, r.income);
});
test("minor-unit rounding conserves every peso and cent", () => {
  assert.deepEqual(distribute(1n, [1n, 1n, 1n]), [1n, 0n, 0n]);
  for (let n = 0n; n < 1000n; n++) assert.equal(distribute(n, [17n, 31n, 52n]).reduce((a,b) => a+b,0n), n);
  const usd = calculate({ ...fixture, currency: "USD", emergency_balance: "100" }, "0,01");
  assert.equal(usd.income, 1n);
  assert.equal(usd.general + usd.wants + usd.index + usd.ownHome + usd.rental + usd.travel, 1n);
  assert.equal(amount(units("999999999999.99", "USD"), "USD"), "999999999999.99");
});
test("invalid amounts, totals, percentages and zero goals rejected", () => {
  for (const v of ["-1", "1e3", "NaN", "1.000.000", "1000000000000", ""]) assert.throws(() => units(v,"USD"));
  assert.throws(() => units("0.01","CLP"));
  for (const patch of [{general:"501"},{reserve:"149"},{emergency_goal:"0"},{reference_income:"0"},{own_home_percent:"101"},{travel_percent:"19.99"}]) assert.throws(() => validateSettings({...fixture,...patch}));
  assert.doesNotThrow(() => validateSettings({...fixture, investment:"0", index_fund:"0", reserve:"0", general:"900"}));
});
test("routes, root alias, OAuth return and unknown pages", () => {
  for (const [page,path] of Object.entries(pagePaths)) assert.equal(pageFromLocation(path,""),page);
  assert.equal(pageFromLocation("/", ""),"dashboard");
  assert.equal(pageFromLocation("/calculadora/", ""),"calculadora");
  assert.equal(pageFromLocation("/", "?gmail=connected"),"ajustes");
  assert.equal(pageFromLocation("/inexistente", ""), null);
});
