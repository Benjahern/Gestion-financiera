import assert from "node:assert/strict";
import { after, test } from "node:test";
import { fileURLToPath } from "node:url";
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import react from "@vitejs/plugin-react";
import { createServer } from "vite";

// Transform the real component without opening a port or contacting the API.
const server = await createServer({
  root: fileURLToPath(new URL("..", import.meta.url)),
  configFile: false,
  plugins: [react()],
  server: { middlewareMode: true, watch: null },
  appType: "custom",
});
after(() => server.close());
const { default: InvestmentAccounts } = await server.ssrLoadModule(
  "/src/components/InvestmentAccounts.tsx",
);
const accounts = [
  {
    id: 1,
    name: "Cartera pesos",
    currency: "CLP",
    type: "Banco",
    opening_balance: "1200",
    balance: "1000",
    is_investment: true,
  },
  {
    id: 2,
    name: "Disponible pesos",
    currency: "CLP",
    type: "Banco",
    opening_balance: "500",
    balance: "500",
    is_investment: false,
  },
  {
    id: 3,
    name: "Cartera dólares",
    currency: "USD",
    type: "Banco",
    opening_balance: "12.34",
    balance: "12.34",
    is_investment: true,
  },
  {
    id: 4,
    name: "Disponible dólares",
    currency: "USD",
    type: "Banco",
    opening_balance: "56.78",
    balance: "56.78",
    is_investment: false,
  },
];
const render = (currency, items = accounts) =>
  renderToStaticMarkup(
    createElement(InvestmentAccounts, {
      accounts: items,
      currency,
      onAccountsChanged: async () => {},
    }),
  );

test("shows classified accounts and only unclassified options in selected currency", () => {
  for (const [currency, included, excluded] of [
    ["CLP", ["Cartera pesos", "Disponible pesos"], ["Cartera dólares", "Disponible dólares"]],
    ["USD", ["Cartera dólares", "Disponible dólares"], ["Cartera pesos", "Disponible pesos"]],
  ]) {
    const html = render(currency);
    for (const name of included) assert.ok(html.includes(name));
    for (const name of excluded) assert.ok(!html.includes(name));
    const options = html.match(/<select[\s\S]*?<\/select>/)[0];
    assert.ok(options.includes(included[1]));
    assert.ok(!options.includes(included[0]));
    assert.match(html, /Quitar de inversiones/);
    assert.match(html, /no cambia su saldo ni crea movimientos/);
    assert.match(html, /independientes del mes seleccionado/);
  }
});

test("renders current balance rather than opening balance", () => {
  const html = render("CLP");
  assert.match(html, /1\.000/);
  assert.doesNotMatch(html, /1\.200/);
});

test("classification is explicit, not inferred again from the type", () => {
  const html = render("CLP", [{ ...accounts[1], type: "inversión" }]);
  assert.match(html, /Aún no has seleccionado cuentas de inversión/);
  assert.match(html, /<option value="2">Disponible pesos/);
  assert.doesNotMatch(html, /Quitar de inversiones/);
});

test("empty currency offers no cross-currency options or enabled submit", () => {
  const html = render("USD", accounts.slice(0, 2));
  assert.match(html, /Aún no has seleccionado cuentas de inversión en USD/);
  assert.match(html, /No hay otras cuentas en USD/);
  assert.match(html, /<button[^>]*type="submit"[^>]*disabled/);
  assert.doesNotMatch(html, /Cartera pesos|Disponible pesos/);
});
