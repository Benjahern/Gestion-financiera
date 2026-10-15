import type { Page } from "./types";

export const pagePaths: Record<Page, string> = {
  dashboard: "/resumen", gastos: "/gastos", ingresos: "/ingresos",
  inversiones: "/inversiones", cuentas: "/cuentas", ajustes: "/conexiones", calculadora: "/calculadora",
};
export function pageFromLocation(path: string, search: string): Page | null {
  if (new URLSearchParams(search).has("gmail")) return "ajustes";
  if (path === "/") return "dashboard";
  const normalized = path.replace(/\/$/, "");
  return (Object.keys(pagePaths) as Page[]).find(page => pagePaths[page] === normalized) ?? null;
}
