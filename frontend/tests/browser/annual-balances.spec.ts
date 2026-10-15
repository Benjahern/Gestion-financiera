import { test, expect, type Page } from "@playwright/test";

async function setup(page: Page, failFirst = false) {
  const history: Record<string,Record<string,string>> = { CLP: {}, USD: {} };
  await page.route("**/api/**", async route => {
    const request = route.request(), url = new URL(request.url());
    const currency = url.searchParams.get("currency") ?? "CLP";
    const match = url.pathname.match(/^\/api\/annual-balances\/(\d+)$/);
    let body: unknown;
    if (match && request.method() !== "GET") {
      expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
      if (request.method() === "PUT") {
        const value = request.postDataJSON(); expect(Object.keys(value).sort()).toEqual(["balance","currency"]);
        history[value.currency][match[1]] = value.balance;
        body = { year: Number(match[1]), balance: value.balance };
      } else { expect(request.method()).toBe("DELETE"); delete history[currency][match[1]]; body = { ok:true }; }
    } else {
      expect(request.method()).toBe("GET");
      if (url.pathname === "/api/auth/session") body = { email:"owner@example.invalid", csrf_token:"synthetic-csrf" };
      else if (url.pathname === "/api/accounts") body = [
        {id:1,name:"Synthetic fund",type:"Inversión",currency:"CLP",opening_balance:"1000",balance:"1000",is_investment:true},
        {id:2,name:"Synthetic savings",type:"Personal",currency:"CLP",opening_balance:"1000",balance:"1000",is_investment:false},
        {id:3,name:"Synthetic USD",type:"Personal",currency:"USD",opening_balance:"0.75",balance:"0.75",is_investment:false},
      ];
      else if (url.pathname === "/api/overview") body = { currency,month:"2026-10",balance:currency === "CLP" ? "2000" : "0.75",income:"0",expense:"0",investment:"0",count:0,categories:[],flow:[] };
      else if (url.pathname === "/api/movements") body = {items:[],total:0,offset:0,limit:5};
      else if (url.pathname === "/api/annual-balances") {
        if (failFirst) { failFirst=false; await route.fulfill({ status:503,json:{ error:"Cierres temporalmente no disponibles" } }); return; }
        body = { currency,current_year:2026,items:Object.entries(history[currency]).map(([year,balance]) => ({ year:Number(year),balance })) };
      } else throw new Error(`Unexpected API: ${request.method()} ${url.pathname}`);
    }
    await route.fulfill({ json:body });
  });
}

test("private annual closings support exact comparisons, gaps, editing and currencies", async ({ page }) => {
  await setup(page); await page.goto("/resumen");
  const panel = page.getByRole("region", { name:"Comparación anual" });
  await expect(panel.getByText("Sin comparación",{exact:true})).toBeVisible();
  const add = async (year:string,balance:string) => {
    await panel.getByRole("button",{name:"Añadir cierre anual"}).click();
    await page.getByLabel("Año del cierre").fill(year);
    await page.getByLabel(/Saldo total al cierre/).fill(balance);
    await page.getByRole("button",{name:"Guardar cierre",exact:true}).click();
    await expect(page.getByRole("dialog")).toHaveCount(0);
  };
  await add("2025","1600");
  await expect(panel.locator(".annual-current")).toContainText("+25,00%");
  await add("2024","1000");
  await expect(panel.getByRole("row").filter({has:page.getByRole("button",{name:"Editar cierre 2025",exact:true})})).toContainText("+60,00%");
  await panel.getByRole("button",{name:"Añadir cierre anual"}).click();
  await page.getByLabel(/Saldo total al cierre/).fill("999");
  page.once("dialog",dialog => void dialog.dismiss());
  await page.getByRole("button",{name:"Guardar cierre",exact:true}).click();
  await expect(panel.locator(".annual-current")).toContainText("+25,00%");
  await page.getByRole("button",{name:"Cancelar",exact:true}).click();
  await add("2022","800");
  await expect(panel.getByText("Falta cierre 2023",{exact:true})).toBeVisible();
  await panel.getByRole("button",{name:"Editar cierre 2024",exact:true}).click();
  await page.getByLabel(/Saldo total al cierre/).fill("0");
  await page.getByRole("button",{name:"Guardar cierre",exact:true}).click();
  await expect(panel.getByText("Base cero o negativa",{exact:true})).toBeVisible();
  await page.reload();
  await expect(panel.locator(".annual-current")).toContainText("+25,00%");
  await page.getByLabel("Moneda",{exact:true}).selectOption("USD");
  await expect(panel.getByText("Sin comparación",{exact:true})).toBeVisible();
  await add("2025","0,50");
  await expect(panel.locator(".annual-current")).toContainText("+50,00%");
  await page.getByRole("button",{name:"Ocultar saldo",exact:true}).click();
  await expect(panel.locator(".annual-current")).not.toContainText("0,75");
  await page.getByRole("button",{name:"Mostrar saldo",exact:true}).click();
  page.once("dialog",dialog => void dialog.accept());
  await panel.getByRole("button",{name:"Eliminar cierre 2025",exact:true}).click();
  await expect(panel.getByText("Sin comparación",{exact:true})).toBeVisible();
  await page.getByLabel("Moneda",{exact:true}).selectOption("CLP");
  await expect(panel.locator(".annual-current")).toContainText("+25,00%");
  expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
  await page.screenshot({ path:test.info().outputPath("annual.png"),fullPage:true });
});

test("history load errors are retryable without hiding the financial summary", async ({ page }) => {
  await setup(page,true); await page.goto("/resumen");
  const panel=page.getByRole("region",{name:"Comparación anual"});
  await expect(panel.getByRole("alert")).toHaveText("Cierres temporalmente no disponibles");
  await expect(page.getByRole("heading",{name:"Tus finanzas en Orden"})).toBeVisible();
  await panel.getByRole("button",{name:"Reintentar cierres"}).click();
  await expect(panel.getByText("Sin comparación",{exact:true})).toBeVisible();
});

test("a failed save keeps the draft and can be retried", async ({ page }) => {
  await setup(page); await page.goto("/resumen");
  await page.getByRole("button",{name:"Añadir cierre anual"}).click();
  await page.getByLabel(/Saldo total al cierre/).fill("1000");
  await page.route("**/api/annual-balances/2025",route => route.fulfill({status:503,json:{error:"No se pudo guardar el cierre de prueba"}}),{times:1});
  await page.getByRole("button",{name:"Guardar cierre",exact:true}).click();
  await expect(page.getByRole("dialog").getByRole("alert")).toContainText("No se pudo guardar");
  await expect(page.getByLabel(/Saldo total al cierre/)).toHaveValue("1000");
  await page.getByRole("button",{name:"Guardar cierre",exact:true}).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.locator(".annual-current")).toContainText("+100,00%");
});
