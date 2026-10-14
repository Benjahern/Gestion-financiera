export class ApiError extends Error {
  constructor(
    message: string,
    public status: number,
  ) {
    super(message);
  }
}

let csrfToken = "";
export function setCSRF(token: string) {
  csrfToken = token;
}

export async function api<T>(
  path: string,
  options: { method?: string; body?: unknown; signal?: AbortSignal } = {},
): Promise<T> {
  const method = options.method ?? "GET";
  const headers: Record<string, string> = { Accept: "application/json" };
  if (method !== "GET") {
    headers["Content-Type"] = "application/json";
    if (csrfToken) headers["X-CSRF-Token"] = csrfToken;
  }
  let response: Response;
  try {
    response = await fetch(`/api${path}`, {
      method,
      headers,
      credentials: "same-origin",
      cache: "no-store",
      signal: options.signal,
      body: method === "GET" ? undefined : JSON.stringify(options.body ?? {}),
    });
  } catch (error) {
    if (error instanceof DOMException && error.name === "AbortError") throw error;
    throw new ApiError(
      "No se pudo conectar con Orden. Comprueba que el backend esté disponible.",
      0,
    );
  }
  const data = await response.json().catch(() => null);
  if (!response.ok) {
    if (response.status === 401 && path !== "/auth/login") {
      csrfToken = "";
      window.dispatchEvent(new Event("orden:session-expired"));
    }
    throw new ApiError(data?.error ?? "No se pudo completar la operación.", response.status);
  }
  if (data === null) throw new ApiError("Respuesta inesperada del servidor.", response.status);
  return data as T;
}
