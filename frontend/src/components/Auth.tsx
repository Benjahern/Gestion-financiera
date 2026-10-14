import { useState, type FormEvent } from "react";
import { api } from "../lib/api";
import type { Session } from "../lib/types";
import { Brand, Button } from "./ui";

type Step = "email" | "login" | "activate" | "recover";
export default function Auth({ onLogin }: { onLogin: (session: Session) => void }) {
  const [step, setStep] = useState<Step>("email");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [confirmation, setConfirmation] = useState("");
  const [code, setCode] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const changeStep = (next: Step) => {
    setStep(next);
    setPassword("");
    setConfirmation("");
    setCode("");
    setError("");
  };
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    setError("");
    if ((step === "activate" || step === "recover") && password !== confirmation) {
      setError("Las contraseñas no coinciden.");
      return;
    }
    setBusy(true);
    try {
      if (step === "email") {
        const result = await api<{ next: "login" | "activate" }>("/auth/start", {
          method: "POST",
          body: { email },
        });
        changeStep(result.next);
      } else {
        const session = await api<Session>(`/auth/${step}`, {
          method: "POST",
          body: {
            email,
            password,
            ...(step !== "login" ? { code: code.trim() } : {}),
          },
        });
        setPassword("");
        setConfirmation("");
        setCode("");
        onLogin(session);
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "No se pudo iniciar sesión.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <main className="auth-shell">
      <section className="auth-card">
        <Brand />
        <div className="eyebrow">Tu espacio financiero privado</div>
        <h1>
          {step === "email"
            ? "Bienvenido a Orden"
            : step === "login"
              ? "Inicia sesión"
              : step === "activate"
                ? "Crea tu contraseña"
                : "Recupera el acceso"}
        </h1>
        <p className="auth-description">
          {step === "email"
            ? "Introduce tu correo para continuar. El acceso está reservado al propietario."
            : step === "login"
              ? "Tu dinero, organizado y bajo tu control."
              : "Utiliza el código generado en tu servidor. Es de un solo uso y caduca en 15 minutos."}
        </p>
        <form onSubmit={submit} className="auth-form">
          <label className="field">
            <span>Correo electrónico</span>
            <input
              type="email"
              value={email}
              onChange={(event) => setEmail(event.target.value)}
              required
              maxLength={254}
              autoComplete="username"
              readOnly={step !== "email"}
              autoFocus
            />
          </label>
          {(step === "activate" || step === "recover") && (
            <label className="field">
              <span>Código de activación del servidor</span>
              <input
                value={code}
                onChange={(event) => setCode(event.target.value)}
                required
                minLength={43}
                maxLength={43}
                autoComplete="off"
                spellCheck={false}
              />
            </label>
          )}
          {step !== "email" && (
            <label className="field">
              <span>
                {step === "login" ? "Contraseña" : "Nueva contraseña (mínimo 12 caracteres)"}
              </span>
              <input
                type="password"
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                required
                minLength={step === "login" ? 1 : 12}
                maxLength={256}
                autoComplete={step === "login" ? "current-password" : "new-password"}
              />
            </label>
          )}
          {(step === "activate" || step === "recover") && (
            <label className="field">
              <span>Confirma la contraseña</span>
              <input
                type="password"
                value={confirmation}
                onChange={(event) => setConfirmation(event.target.value)}
                required
                minLength={12}
                maxLength={256}
                autoComplete="new-password"
              />
            </label>
          )}
          {error && (
            <div className="error-message" role="alert">
              {error}
            </div>
          )}
          <Button className="primary-button" type="submit" disabled={busy}>
            {busy
              ? "Comprobando…"
              : step === "email"
                ? "Continuar"
                : step === "login"
                  ? "Entrar a Orden"
                  : "Guardar contraseña y entrar"}
          </Button>
        </form>
        {step === "login" && (
          <Button className="text-button" disabled={busy} onClick={() => changeStep("recover")}>
            Recuperar acceso con un código del servidor
          </Button>
        )}
        {step !== "email" && (
          <Button className="text-button" disabled={busy} onClick={() => changeStep("email")}>
            Volver al correo
          </Button>
        )}
        <div className="auth-footer">Sin registro público · Sesión privada</div>
      </section>
    </main>
  );
}
