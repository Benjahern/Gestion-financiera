import { useEffect, useState, type FormEvent } from "react";
import { api } from "../lib/api";
import type { Account } from "../lib/types";
import { Button } from "./ui";

type Category = { id: number; name: string };
type Gmail = {
  configured: boolean;
  connected: boolean;
  email?: string;
  last_sync?: string;
  last_error?: string;
  rules: { id: number; bank: string; last_four: string; account_id: number }[];
};
export default function Settings({
  accounts,
  onData,
}: {
  accounts: Account[];
  onData: () => Promise<void>;
}) {
  const [gmail, setGmail] = useState<Gmail | null>(null);
  const [categories, setCategories] = useState<Category[]>([]);
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [lastFour, setLastFour] = useState("");
  const [accountID, setAccountID] = useState("");
  const [name, setName] = useState("");
  const load = async () => {
    const [g, c] = await Promise.all([api<Gmail>("/gmail"), api<Category[]>("/categories")]);
    setGmail(g);
    setCategories(c);
  };
  useEffect(() => {
    void load().catch((e) => setError(e.message));
    const params = new URLSearchParams(window.location.search);
    if (params.has("gmail")) {
      setNotice(
        params.get("gmail") === "connected"
          ? "Gmail conectado correctamente."
          : "No se completó la autorización. Puedes intentarlo nuevamente.",
      );
      params.delete("gmail");
      window.history.replaceState(
        null,
        "",
        window.location.pathname + (params.size ? "?" + params : ""),
      );
    }
  }, []);
  const action = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "No se pudo completar la acción");
    } finally {
      setBusy(false);
    }
  };
  const connect = () =>
    action(async () => {
      const r = await api<{ url: string }>("/gmail/connect", { method: "POST" });
      window.location.assign(r.url);
    });
  const addRule = (e: FormEvent) => {
    e.preventDefault();
    void action(async () => {
      await api("/gmail/rules", {
        method: "POST",
        body: { bank: "mach", last_four: lastFour, account_id: Number(accountID) },
      });
      setLastFour("");
    });
  };
  const addCategory = (e: FormEvent) => {
    e.preventDefault();
    void action(async () => {
      await api("/categories", { method: "POST", body: { name: name.trim() } });
      setName("");
    });
  };
  return (
    <div className="page settings-page">
      <div className="page-intro">
        <div>
          <div className="eyebrow">Tú decides qué conectar</div>
          <h1 className="page-title">Conexiones y categorías</h1>
          <p className="page-subtitle">
            Correo solo para compras. Ingresos, ahorro e inversiones siguen siendo manuales.
          </p>
        </div>
      </div>
      {error && (
        <div className="error-message" role="alert">
          {error}
        </div>
      )}
      {notice && <p role="status">{notice}</p>}
      <section className="panel">
        <h2>Gmail · conexión opcional</h2>
        <p className="form-note">
          Google solicita permiso de lectura del correo completo: no ofrece uno limitado a compras.
          Orden solo procesa avisos compatibles, no envía mensajes ni importa otros movimientos. Se
          revisa cada cinco minutos desde la fecha de conexión, sin importar el historial anterior.
        </p>
        {!gmail ? (
          <p>Cargando conexión…</p>
        ) : (
          <>
            {!gmail.configured && (
              <p className="form-note">
                Faltan las credenciales OAuth del backend. Consulta deploy/README.md; el resto de
                Orden funciona sin conectar Gmail.
              </p>
            )}
            {gmail.connected && (
              <p>
                Cuenta conectada: <strong>{gmail.email}</strong>
                {gmail.last_sync && (
                  <> · Última revisión: {new Date(gmail.last_sync).toLocaleString("es-CL")}</>
                )}
              </p>
            )}
            {gmail.last_error && <p role="status">{gmail.last_error}</p>}
            <div className="settings-actions">
              <Button
                className="primary-button"
                disabled={busy || !gmail.configured}
                onClick={() => void connect()}
              >
                {gmail.connected ? "Volver a autorizar Gmail" : "Conectar Gmail con Google"}
              </Button>
              {gmail.connected && (
                <>
                  <Button
                    className="secondary-button"
                    disabled={busy}
                    onClick={() =>
                      void action(async () => {
                        const r = await api<{ imported: number }>("/gmail/sync", {
                          method: "POST",
                        });
                        setNotice(
                          `${r.imported} compras importadas. Los gastos están pendientes de descripción.`,
                        );
                        await onData();
                      })
                    }
                  >
                    Revisar compras ahora
                  </Button>
                  <Button
                    className="text-button danger"
                    disabled={busy}
                    onClick={() => {
                      if (
                        window.confirm(
                          "¿Desconectar Gmail? Se detiene la lectura; tus gastos se conservan.",
                        )
                      )
                        void action(async () => {
                          const r = await api<{ revoked: boolean }>("/gmail", { method: "DELETE" });
                          setNotice(
                            r.revoked
                              ? "Gmail desconectado y permiso revocado."
                              : "Lectura detenida. Revoca también el permiso desde tu cuenta de Google si no pudo confirmarse la revocación.",
                          );
                        });
                    }}
                  >
                    Desconectar
                  </Button>
                </>
              )}
            </div>
          </>
        )}
      </section>
      <section className="panel">
        <h2>Asociar tarjeta a una cuenta</h2>
        <p className="form-note">
          MACHBANK: compras CLP del formato confirmado. Bci queda desactivado hasta disponer de un
          correo de ejemplo. Los pagos de la tarjeta no se importan como gastos.
        </p>
        <form className="settings-form" onSubmit={addRule}>
          <label className="field">
            <span>Últimos cuatro dígitos de MACH</span>
            <input
              value={lastFour}
              onChange={(e) => setLastFour(e.target.value)}
              inputMode="numeric"
              pattern="[0-9]{4}"
              maxLength={4}
              required
            />
          </label>
          <label className="field">
            <span>Cuenta para registrar el gasto</span>
            <select value={accountID} onChange={(e) => setAccountID(e.target.value)} required>
              <option value="">Selecciona una cuenta CLP</option>
              {accounts
                .filter((a) => a.currency === "CLP")
                .map((a) => (
                  <option key={a.id} value={a.id}>
                    {a.name}
                  </option>
                ))}
            </select>
          </label>
          <Button className="primary-button" type="submit" disabled={busy}>
            Guardar asociación
          </Button>
        </form>
        {gmail?.rules.map((rule) => (
          <div className="settings-row" key={rule.id}>
            <span>
              MACH · termina en {rule.last_four} →{" "}
              {accounts.find((a) => a.id === rule.account_id)?.name ?? "Cuenta"}
            </span>
            <Button
              className="text-button danger"
              disabled={busy}
              onClick={() =>
                void action(async () => {
                  await api(`/gmail/rules/${rule.id}`, { method: "DELETE" });
                })
              }
            >
              Quitar
            </Button>
          </div>
        ))}
      </section>
      <section className="panel">
        <h2>Tus categorías</h2>
        <p className="form-note">
          Categorías iniciales editables. Usa “Categoría &gt; Subcategoría” cuando corresponda.
          El clasificador aprende únicamente de tus confirmaciones y correcciones; las predicciones
          sin confirmar nunca lo entrenan.
        </p>
        <form className="settings-form" onSubmit={addCategory}>
          <label className="field">
            <span>Nueva categoría</span>
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              maxLength={100}
              required
              placeholder="Transporte > Bicicleta"
            />
          </label>
          <Button type="submit" className="primary-button" disabled={busy}>
            Añadir
          </Button>
        </form>
        {categories.map((c) => (
          <div key={c.id} className="settings-row">
            <span>{c.name}</span>
            <div>
              <Button
                className="text-button"
                disabled={busy}
                onClick={() => {
                  const name = window.prompt("Nombre de categoría", c.name);
                  if (name && name !== c.name)
                    void action(async () => {
                      await api(`/categories/${c.id}`, { method: "PUT", body: { name } });
                      await onData();
                    });
                }}
              >
                Editar
              </Button>
              <Button
                className="text-button danger"
                disabled={busy}
                onClick={() => {
                  if (
                    window.confirm(
                      `¿Eliminar ${c.name}? Solo se pueden eliminar categorías sin movimientos.`,
                    )
                  )
                    void action(async () => {
                      await api(`/categories/${c.id}`, { method: "DELETE" });
                    });
                }}
              >
                Eliminar
              </Button>
            </div>
          </div>
        ))}
      </section>
    </div>
  );
}
