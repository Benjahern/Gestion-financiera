import { test, expect } from "@playwright/test";

// All browser API responses are synthetic: these tests never contact a real API.
const config = {
  currency: "CLP",
  reference_income: "1000",
  general: "500",
  wants: "100",
  investment: "400",
  index_fund: "250",
  reserve: "150",
  emergency_goal: "300000",
  emergency_balance: "0",
  own_home_percent: "50",
  rental_percent: "30",
  travel_percent: "20",
};

test.beforeEach(async ({ page }) => {
  const saved: Record<string, unknown> = { CLP: { ...config } };
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
          name: "Synthetic savings",
          type: "Personal",
          currency: "CLP",
          opening_balance: "99.00",
          balance: "99.00",
        },
      ];
    else if (url.pathname === "/api/calculator") {
      if (request.method() === "PUT") {
        const value = request.postDataJSON();
        expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
        expect(value).toMatchObject({
          ...config,
          emergency_goal: "200",
          own_home_percent: "50.00",
          rental_percent: "30.00",
          travel_percent: "20.00",
        });
        saved[value.currency] = value;
        body = { settings: value };
      } else body = { settings: saved[url.searchParams.get("currency") ?? ""] ?? null };
    } else if (url.pathname === "/api/overview")
      body = {
        currency: "CLP",
        month: "2026-01",
        balance: "0",
        income: "0",
        expense: "0",
        investment: "0",
        count: 0,
        categories: [],
        flow: [],
      };
    else if (url.pathname === "/api/movements")
      body = { items: [], total: 0, offset: 0, limit: 50 };
    else if (url.pathname === "/api/gmail")
      body = { configured: false, connected: false, rules: [] };
    else if (url.pathname === "/api/categories") body = [];
    else throw new Error(`Unexpected API call: ${request.method()} ${url.pathname}`);
    if (request.method() !== "GET" && url.pathname !== "/api/calculator")
      throw new Error("Calculator must not mutate financial data");
    await route.fulfill({ json: body });
  });
});

test("calculator asks only for the amount and keeps Excel rules fixed", async ({ page }) => {
  await page.goto("/calculadora");
  await expect(page.getByRole("heading", { name: "Calculadora mensual" })).toBeVisible();
  await expect(page.getByLabel("Dinero a repartir · CLP")).toBeVisible();
  await expect(page.getByLabel("Sueldo de referencia · CLP")).toHaveCount(0);
  await page.getByLabel("Dinero a repartir · CLP").fill("1000");
  await expect(page.getByText("En construcción", { exact: false })).toBeVisible();
  const results = page.getByLabel("Reparto del ingreso");
  await expect(results.getByText("Fondo de emergencia", { exact: true })).toBeVisible();
  await expect(
    results.locator(".calculator-result").filter({ hasText: "Fondo de emergencia" }),
  ).toContainText("150");
  const settings = page.getByText("Editar meta del fondo de emergencia");
  await settings.click();
  await page.getByLabel("Meta del fondo · CLP").fill("200");
  await page.getByRole("button", { name: "Guardar meta" }).click();
  await expect(
    page.getByRole("status").filter({ hasText: "Configuración guardada" }),
  ).toBeVisible();
  await page.screenshot({ path: test.info().outputPath("calculator.png"), fullPage: true });
  await page.reload();
  await expect(page.getByLabel("Meta del fondo · CLP")).toHaveValue("200");
  await expect(page.getByLabel("Dinero a repartir · CLP")).toHaveValue("");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(
    true,
  );
});

test("deep links, back/forward, reload and Gmail return preserve routes", async ({ page }) => {
  await page.goto("/calculadora");
  await page.getByRole("link", { name: "Gastos", exact: true }).click();
  await expect(page).toHaveURL(/\/gastos$/);
  await page.reload();
  await expect(page.getByRole("heading", { name: "Gastos", exact: true })).toBeVisible();
  await page.goBack();
  await expect(page).toHaveURL(/\/calculadora$/);
  await expect(page.getByRole("heading", { name: "Calculadora mensual" })).toBeVisible();
  await page.goForward();
  await expect(page).toHaveURL(/\/gastos$/);
  await page.goto("/?gmail=connected");
  await expect(page.getByRole("heading", { name: "Conexiones y categorías" })).toBeVisible();
  await expect(page).toHaveURL(/\/conexiones$/);
  await page.goto("/no-existe");
  await expect(page.getByRole("heading", { name: "Página no encontrada" })).toBeVisible();
});
