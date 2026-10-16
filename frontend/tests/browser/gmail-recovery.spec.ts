import { test, expect, type Page } from "@playwright/test";

async function setup(page: Page, failRecovery = false) {
  let recoveries = 0;
  await page.route("**/api/**", async (route) => {
    const request = route.request();
    const url = new URL(request.url());
    let body: unknown;
    if (request.method() === "POST") {
      expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
      if (url.pathname === "/api/gmail/sync") body = { imported: 0 };
      else if (url.pathname === "/api/gmail/rescan") {
        recoveries++;
        if (failRecovery && recoveries === 1) {
          await route.fulfill({ status: 503, json: { error: "Sincronización incompleta; vuelve a intentarlo" } });
          return;
        }
        body = { imported: 1 };
      } else throw new Error(`Unexpected mutation ${url.pathname}`);
    } else if (url.pathname === "/api/auth/session") {
      body = { email: "owner@example.invalid", csrf_token: "synthetic-csrf" };
    } else if (url.pathname === "/api/accounts") {
      body = [{ id: 1, name: "Synthetic account", type: "Personal", currency: "CLP", balance: "1000", opening_balance: "1000", is_investment: false, show_in_summary: false }];
    } else if (url.pathname === "/api/gmail") {
      body = { configured: true, connected: true, email: "owner@example.invalid", rules: [{ id: 1, bank: "mach", last_four: "1234", account_id: 1 }] };
    } else if (url.pathname === "/api/categories") body = [];
    else if (url.pathname === "/api/overview") {
      body = { currency: "CLP", month: url.searchParams.get("month"), balance: "1000", income: "0", expense: "0", investment: "0", count: 0, categories: [], flow: [] };
    } else if (url.pathname === "/api/movements") body = { items: [], total: 0, offset: 0, limit: 5 };
    else if (url.pathname === "/api/annual-balances") body = { currency: "CLP", current_year: 2026, items: [] };
    else throw new Error(`Unexpected API ${url.pathname}`);
    await route.fulfill({ json: body });
  });
  return () => recoveries;
}

test("zero imports is not described as pending; recovery requires explicit confirmation", async ({ page }) => {
  const recoveries = await setup(page);
  await page.goto("/conexiones");
  await expect(page.getByText("MACH · termina en 1234 → Synthetic account")).toBeVisible();
  await page.getByRole("button", { name: "Revisar compras ahora", exact: true }).click();
  await expect(page.getByRole("status")).toContainText("No se importaron compras nuevas");
  await expect(page.getByRole("status")).not.toContainText("pendientes de descripción");
  const recover = page.getByRole("button", { name: "Recuperar compras desde la conexión" });
  page.once("dialog", async (dialog) => { await dialog.dismiss(); });
  await recover.click();
  expect(recoveries()).toBe(0);
  page.once("dialog", async (dialog) => {
    expect(dialog.message()).toContain("No se duplicarán gastos");
    await dialog.accept();
  });
  await recover.click();
  await expect(page.getByRole("status")).toContainText("1 compra importada en este lote");
  expect(recoveries()).toBe(1);
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
});

test("failed recovery is visible and can be retried", async ({ page }) => {
  await setup(page, true);
  await page.goto("/conexiones");
  const recover = page.getByRole("button", { name: "Recuperar compras desde la conexión" });
  page.on("dialog", async (dialog) => { await dialog.accept(); });
  await recover.click();
  await expect(page.getByRole("alert")).toContainText("Sincronización incompleta");
  await expect(recover).toBeEnabled();
  await recover.click();
  await expect(page.getByRole("alert")).toHaveCount(0);
  await expect(page.getByRole("status")).toContainText("1 compra importada en este lote");
});
