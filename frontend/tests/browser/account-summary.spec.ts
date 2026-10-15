import { test, expect, type Page } from "@playwright/test";

async function setup(page:Page, failFirst=false) {
  // Synthetic account balances, never a production API or personal workbook.
  const accounts = [
    {id:1,name:"Synthetic liquid",type:"Personal",currency:"CLP",opening_balance:"1000",balance:"1234",is_investment:false,show_in_summary:false},
    {id:2,name:"Synthetic fund",type:"Inversión",currency:"CLP",opening_balance:"50",balance:"50",is_investment:true,show_in_summary:false},
    {id:3,name:"Synthetic USD",type:"Personal",currency:"USD",opening_balance:"1",balance:"1.23",is_investment:false,show_in_summary:false},
  ];
  await page.route("**/api/**",async route => {
    const request=route.request(),url=new URL(request.url());
    const currency=url.searchParams.get("currency")??"CLP";
    let body:unknown;
    const match=url.pathname.match(/^\/api\/accounts\/(\d+)\/summary$/);
    if(match){
      expect(request.method()).toBe("PUT");expect(request.headers()["x-csrf-token"]).toBe("synthetic-csrf");
      const value=request.postDataJSON();expect(Object.keys(value)).toEqual(["show_in_summary"]);
      if(failFirst){failFirst=false;await route.fulfill({status:503,json:{error:"No se pudo guardar la selección"}});return;}
      const target=accounts.find(a=>a.id===Number(match[1]))!;
      if(value.show_in_summary) for(const a of accounts) if(a.currency===target.currency)a.show_in_summary=false;
      target.show_in_summary=value.show_in_summary;body={ok:true};
    } else {
      expect(request.method()).toBe("GET");
      if(url.pathname==="/api/auth/session")body={email:"owner@example.invalid",csrf_token:"synthetic-csrf"};
      else if(url.pathname==="/api/accounts")body=accounts;
      else if(url.pathname==="/api/overview")body={currency,month:url.searchParams.get("month"),balance:currency==="CLP"?"1284":"1.23",income:"0",expense:"0",investment:"0",count:0,categories:[],flow:[]};
      else if(url.pathname==="/api/movements")body={items:[],total:0,offset:0,limit:5};
      else if(url.pathname==="/api/annual-balances")body={currency,current_year:2026,items:[]};
      else throw new Error(`Unexpected API: ${request.method()} ${url.pathname}`);
    }
    await route.fulfill({json:body});
  });
}

test("saved account appears next to general balance, masked and isolated by currency",async({page})=>{
  await setup(page);await page.goto("/cuentas");
  const liquid=page.getByRole("checkbox",{name:"Mostrar en Resumen: Synthetic liquid",exact:true});
  const fund=page.getByRole("checkbox",{name:"Mostrar en Resumen: Synthetic fund",exact:true});
  await expect(liquid).not.toBeChecked();await liquid.click();await expect(liquid).toBeChecked();
  await page.getByRole("link",{name:"Resumen",exact:true}).click();
  const card=page.getByRole("region",{name:"Cuenta destacada"});
  await expect(card).toContainText("Synthetic liquid");await expect(card).toContainText("CLP 1.234");
  await expect(page.locator(".wealth-copy .wealth-value")).toContainText("1.284");
  await expect(card).toContainText("ya incluido en el saldo general");
  await page.getByRole("button",{name:"Ocultar saldo",exact:true}).click();
  await expect(card.locator(".wealth-value")).toHaveText("••••••");
  await expect(page.locator(".wealth-copy .wealth-value")).toHaveText("••••••");
  await page.getByRole("button",{name:"Mostrar saldo",exact:true}).click();
  await page.reload();await expect(card).toContainText("CLP 1.234");
  await page.getByLabel("Mes del resumen").fill("2025-12");
  await expect(page.locator(".page-subtitle").first()).toContainText("dic 2025");
  await expect(card).toContainText("CLP 1.234");
  await page.evaluate(()=>window.scrollTo(0,0));
  await page.screenshot({path:test.info().outputPath("summary.png"),fullPage:true});
  expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  await page.getByRole("link",{name:"Cuentas",exact:true}).click();await fund.click();await expect(fund).toBeChecked();await expect(liquid).not.toBeChecked();
  await page.getByLabel("Moneda",{exact:true}).selectOption("USD");
  const usd=page.getByRole("checkbox",{name:"Mostrar en Resumen: Synthetic USD",exact:true});
  await usd.click();await expect(usd).toBeChecked();
  await page.getByRole("link",{name:"Resumen",exact:true}).click();await expect(card).toContainText("USD 1,23");
  await page.getByLabel("Moneda",{exact:true}).selectOption("CLP");await expect(card).toContainText("Synthetic fund");
  await page.getByRole("link",{name:"Cuentas",exact:true}).click();await fund.click();await expect(fund).not.toBeChecked();
  await page.getByRole("link",{name:"Resumen",exact:true}).click();await expect(card).toHaveCount(0);
});

test("failed selection stays unchecked and can be retried",async({page})=>{
  await setup(page,true);await page.goto("/cuentas");
  const check=page.getByRole("checkbox",{name:"Mostrar en Resumen: Synthetic liquid",exact:true});
  await check.click();await expect(page.getByRole("alert")).toContainText("No se pudo guardar la selección");await expect(check).not.toBeChecked();
  await check.click();await expect(check).toBeChecked();
  await page.getByRole("link",{name:"Resumen",exact:true}).click();await expect(page.getByRole("region",{name:"Cuenta destacada"})).toContainText("Synthetic liquid");
});
