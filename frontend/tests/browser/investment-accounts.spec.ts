import { test, expect } from "@playwright/test";

test("existing accounts can enter and leave investments without changing balances", async ({
  page,
}) => {
  const accounts = [
    {
      id: 1,
      name: "Synthetic savings",
      type: "Personal",
      currency: "CLP",
      opening_balance: "99.00",
      balance: "99.00",
      is_investment: false,
    },
    {
      id: 2,
      name: "Synthetic fund",
      type: "Inversión",
      currency: "CLP",
      opening_balance: "300.00",
      balance: "250.00",
      is_investment: true,
    },
    {
      id: 3,
      name: "Synthetic USD",
      type: "Personal",
      currency: "USD",
      opening_balance: "500.00",
      balance: "500.00",
      is_investment: false,
    },
  ];
  const amounts = accounts.map((a) => [a.opening_balance, a.balance]);
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    let body: unknown;
    const match = url.pathname.match(/^\/api\/accounts\/(\d+)\/investment$/);
    if (match && request.method() === "PUT") {
      expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
      const update = request.postDataJSON();
      expect(Object.keys(update)).toEqual(["is_investment"]);
      accounts.find((a) => a.id === Number(match[1]))!.is_investment = update.is_investment;
      body = { ok: true };
    } else {
      expect(request.method()).toBe("GET");
      if (url.pathname === "/api/auth/session")
        body = { email: "owner@example.invalid", csrf_token: "synthetic-csrf" };
      else if (url.pathname === "/api/accounts") body = accounts;
      else if (url.pathname === "/api/overview")
        body = {
          currency: url.searchParams.get("currency"),
          month: "2026-01",
          balance: "0",
          income: "0",
          expense: "0",
          investment: "25",
          count: 0,
          categories: [],
          flow: [
            { month: "2025-12", income: "0", expense: "0", investment: "10" },
            { month: "2026-01", income: "0", expense: "0", investment: "25" },
          ],
        };
      else if (url.pathname === "/api/movements")
        body = { items: [], total: 0, offset: 0, limit: 50 };
      else throw new Error(`Unexpected API call: ${url.pathname}`);
    }
    await route.fulfill({ json: body });
  });
  await page.goto("/inversiones");
  const panel = page.getByRole("region", { name: "Mis cuentas de inversión" });
  const summary = page.getByRole("region", { name: "Panorama de inversiones" });
  await expect(
    summary.locator(".stat-card").filter({ hasText: "Total en inversiones" }),
  ).toContainText("250");
  await expect(
    summary.locator(".stat-card").filter({ hasText: "De tu dinero total" }),
  ).toContainText("71,63%");
  await expect(summary.getByRole("region", { name: "Evolución de aportes" })).toContainText("25");
  await page.screenshot({ path: test.info().outputPath("investments.png"), fullPage: true });
  await expect(panel.locator(".account-card")).toContainText("250");
  await expect(panel.getByRole("option", { name: "Synthetic USD · USD" })).toHaveCount(0);
  await panel.getByLabel("Añadir una cuenta existente (CLP)").selectOption("1");
  await panel.getByRole("button", { name: "Añadir a inversiones" }).click();
  await expect(
    panel.locator(".account-card").filter({ hasText: "Synthetic savings" }),
  ).toContainText("99");
  await expect(
    summary.locator(".stat-card").filter({ hasText: "Total en inversiones" }),
  ).toContainText("349");
  await expect(
    summary.locator(".stat-card").filter({ hasText: "De tu dinero total" }),
  ).toContainText("100,00%");
  await page.reload();
  await expect(panel.getByText("Synthetic savings", { exact: true })).toBeVisible();
  await panel
    .getByRole("button", { name: "Quitar Synthetic savings de inversiones sin borrar la cuenta" })
    .click();
  await expect(panel.getByText("Synthetic savings", { exact: true })).toHaveCount(0);
  await expect(panel.getByRole("option", { name: "Synthetic savings · CLP" })).toHaveCount(1);
  await page.getByLabel("Moneda", { exact: true }).selectOption("USD");
  await expect(panel.getByLabel("Añadir una cuenta existente (USD)")).toBeVisible();
  await expect(panel.getByRole("option", { name: "Synthetic savings · CLP" })).toHaveCount(0);
  await panel.getByLabel("Añadir una cuenta existente (USD)").selectOption("3");
  await panel.getByRole("button", { name: "Añadir a inversiones" }).click();
  await expect(panel.locator(".account-card")).toContainText("500");
  expect(accounts.map((a) => [a.opening_balance, a.balance])).toEqual(amounts);
});

test("investment form separates destination from optional same-currency source", async ({
  page,
}) => {
  let saved: Record<string, unknown> | null = null;
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    let body: unknown;
    if (url.pathname === "/api/auth/session")
      body = { email: "owner@example.invalid", csrf_token: "synthetic-csrf" };
    else if (url.pathname === "/api/accounts")
      body = [
        {
          id: 1,
          name: "Synthetic checking",
          type: "Personal",
          currency: "CLP",
          opening_balance: "1000",
          balance: "1000",
          is_investment: false,
          show_in_summary: false,
        },
        {
          id: 2,
          name: "Synthetic portfolio",
          type: "Inversión",
          currency: "CLP",
          opening_balance: "0",
          balance: "0",
          is_investment: true,
          show_in_summary: false,
        },
        {
          id: 3,
          name: "Synthetic USD",
          type: "Personal",
          currency: "USD",
          opening_balance: "10",
          balance: "10",
          is_investment: false,
          show_in_summary: false,
        },
      ];
    else if (url.pathname === "/api/overview")
      body = {
        currency: "CLP",
        month: "2026-10",
        balance: "1000",
        income: "0",
        expense: "0",
        investment: "0",
        count: 0,
        categories: [],
        flow: [],
      };
    else if (url.pathname === "/api/movements" && request.method() === "POST") {
      expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
      saved = request.postDataJSON();
      body = { id: 9 };
    } else if (url.pathname === "/api/movements")
      body = { items: [], total: 0, offset: 0, limit: 50 };
    else if (url.pathname === "/api/gmail")
      body = { configured: false, connected: false, rules: [] };
    else throw new Error(`Unexpected API call: ${request.method()} ${url.pathname}`);
    await route.fulfill({ json: body });
  });
  await page.goto("/inversiones");
  await page.getByRole("button", { name: "Añadir aporte" }).click();
  await page.getByRole("textbox", { name: "Concepto" }).fill("Synthetic contribution");
  await page.getByLabel("Origen del dinero").selectOption("1");
  await page.getByLabel("Destino de inversión").selectOption("2");
  await page.getByLabel("Importe (CLP)").fill("250");
  await page.getByRole("textbox", { name: "Categoría", exact: true }).fill("Inversión");
  await page.getByRole("button", { name: "Guardar movimiento" }).click();
  await expect
    .poll(() => saved)
    .toMatchObject({
      account_id: 2,
      source_account_id: 1,
      investment_mode: "contribution",
      amount: "250",
      kind: "investment",
    });
});
