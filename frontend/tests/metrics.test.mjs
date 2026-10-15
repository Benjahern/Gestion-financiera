import test from "node:test";
import assert from "node:assert/strict";
import { annualChange, exactMoney, minor, percent, portfolio } from "../src/lib/metrics.ts";

test("annual change is exact, signed and not fabricated without a positive base", () => {
  assert.deepEqual(annualChange(125n,100n), { difference:25n, percentage:2500n });
  assert.equal(percent(annualChange(75n,100n).percentage,true),"−25,00%");
  assert.deepEqual(annualChange(75n,undefined), { difference:null, percentage:null });
  for (const base of [0n,-1n]) assert.equal(annualChange(75n,base).percentage,null);
  assert.equal(annualChange(100n,100n).percentage,0n);
  assert.equal(minor("-0.01","USD"),-1n);
  assert.equal(exactMoney(minor("999999999999.99","USD"),"USD"),"USD\u00a0999.999.999.999,99");
  assert.throws(() => minor("0.01","CLP"));
});
test("portfolio totals only selected investment accounts in the same currency", () => {
  const account = (id,balance,currency,selected) => ({id,name:`Synthetic ${id}`,balance,currency,is_investment:selected});
  const accounts=[account(1,"250.00","CLP",true),account(2,"750.00","CLP",true),account(3,"1000.00","CLP",false),account(4,"500.25","USD",true)];
  const p = portfolio(accounts,"CLP");
  assert.equal(p.total,1000n); assert.equal(p.totalMoney,2000n); assert.equal(p.share,5000n);
  assert.equal(p.entries[0].account.id,2); assert.equal(p.canDistribute,true);
  assert.equal(portfolio(accounts,"USD").total,50025n);
  assert.equal(portfolio([],"CLP").share,null);
  const debt=portfolio([...accounts,account(5,"-100.00","CLP",true)],"CLP");
  assert.equal(debt.total,900n); assert.equal(debt.canDistribute,false);
});
