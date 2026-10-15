import { useEffect, useRef, type ReactNode, type ButtonHTMLAttributes } from "react";

export type IconName =
  | "home"
  | "expense"
  | "income"
  | "investment"
  | "wallet"
  | "calculator"
  | "plus"
  | "eye"
  | "eyeOff"
  | "search"
  | "chevron"
  | "check"
  | "food"
  | "transport"
  | "play"
  | "sun"
  | "user"
  | "gift"
  | "tag"
  | "close";
const paths: Record<IconName, ReactNode> = {
  calculator: <><rect x="5" y="2" width="14" height="20" rx="2" /><path d="M8 6h8v4H8zM8 14h1m6 0h1m-8 4h1m6 0h1" /></>,
  home: (
    <>
      <path d="M3 10.8 12 3l9 7.8" />
      <path d="M5.5 9.5V21h13V9.5M9 21v-7h6v7" />
    </>
  ),
  expense: (
    <>
      <path d="M4 7h16v12H4z" />
      <path d="M4 10h16M8 15h3" />
    </>
  ),
  income: (
    <>
      <path d="M12 3v13M7 11l5 5 5-5" />
      <path d="M5 21h14" />
    </>
  ),
  investment: (
    <>
      <path d="M4 20V10M10 20V4M16 20v-7M22 20H2" />
      <path d="m4 7 6-4 6 7 5-5" />
    </>
  ),
  wallet: (
    <>
      <path d="M3 6.5A2.5 2.5 0 0 1 5.5 4H19v16H5.5A2.5 2.5 0 0 1 3 17.5z" />
      <path d="M3 7h16M15 12h7v5h-7a2.5 2.5 0 0 1 0-5Z" />
    </>
  ),
  plus: <path d="M12 5v14M5 12h14" />,
  eye: (
    <>
      <path d="M2.5 12s3.5-6 9.5-6 9.5 6 9.5 6-3.5 6-9.5 6-9.5-6-9.5-6Z" />
      <circle cx="12" cy="12" r="2.5" />
    </>
  ),
  eyeOff: (
    <>
      <path d="m3 3 18 18M10.6 6.2A9.7 9.7 0 0 1 12 6c6 0 9.5 6 9.5 6a15 15 0 0 1-2.2 2.8M6.3 6.3A15 15 0 0 0 2.5 12s3.5 6 9.5 6c1.5 0 2.8-.4 3.9-.9" />
    </>
  ),
  close: <path d="m6 6 12 12M18 6 6 18" />,
  search: (
    <>
      <circle cx="10.5" cy="10.5" r="6.5" />
      <path d="m16 16 5 5" />
    </>
  ),
  chevron: <path d="m8 10 4 4 4-4" />,
  check: <path d="m5 12 4 4L19 6" />,
  food: (
    <>
      <path d="M5 3v7m3-7v7M2 3v7c0 3 6 3 6 0M5 13v8M17 3c-4 4-4 10 0 10h2M19 3v18" />
    </>
  ),
  transport: (
    <>
      <rect x="5" y="3" width="14" height="16" rx="3" />
      <path d="M5 11h14M8 7h8M8 19l-2 3m10-3 2 3M8 15h1m6 0h1" />
    </>
  ),
  play: (
    <>
      <rect x="3" y="5" width="18" height="14" rx="4" />
      <path d="m10 9 5 3-5 3Z" />
    </>
  ),
  sun: (
    <>
      <circle cx="12" cy="12" r="4" />
      <path d="M12 2v2m0 16v2M2 12h2m16 0h2M5 5l1 1m12 12 1 1M5 19l1-1M18 6l1-1" />
    </>
  ),
  user: (
    <>
      <circle cx="12" cy="7" r="4" />
      <path d="M4 21v-2a8 8 0 0 1 16 0v2" />
    </>
  ),
  gift: (
    <>
      <path d="M4 11v10h16V11M12 7v14M3 7h18v4H3Z" />
      <path d="M12 7S4 7 6 3s6 4 6 4 8 0 6-4-6 4-6 4Z" />
    </>
  ),
  tag: (
    <>
      <path d="M3 3h8l10 10-8 8L3 11Z" />
      <circle cx="7" cy="7" r="1" />
    </>
  ),
};
export function Icon({ name, size = 20 }: { name: IconName; size?: number }) {
  return (
    <svg
      aria-hidden="true"
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      {paths[name]}
    </svg>
  );
}
export function Brand() {
  return (
    <div className="brand">
      <div className="brand-mark">
        <span />
        <span />
        <span />
      </div>
      <div className="brand-name">Orden</div>
    </div>
  );
}
export function Button({ type = "button", ...props }: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button type={type} {...props} />;
}
export function EmptyState({ children }: { children: ReactNode }) {
  return <div className="empty-state">{children}</div>;
}
export function SectionHeader({ title, subtitle }: { title: string; subtitle?: string }) {
  return (
    <div className="section-header">
      <div>
        <div className="section-title">{title}</div>
        {subtitle && <div className="section-subtitle">{subtitle}</div>}
      </div>
    </div>
  );
}
export function StatCard({
  label,
  amount,
  note,
  icon,
}: {
  label: string;
  amount: string;
  note: string;
  icon: IconName;
}) {
  return (
    <div className="stat-card">
      <div className="stat-head">
        <span>{label}</span>
        <div className="icon-tile">
          <Icon name={icon} size={18} />
        </div>
      </div>
      <div className="stat-value">{amount}</div>
      <div className="stat-note">{note}</div>
    </div>
  );
}
export function Modal({
  title,
  onClose,
  children,
}: {
  title: string;
  onClose: () => void;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const close = useRef(onClose);
  close.current = onClose;
  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null;
    const element = ref.current;
    const originalOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    element?.querySelector<HTMLElement>("input,select,button")?.focus();
    const handler = (event: KeyboardEvent) => {
      if (event.key === "Escape") close.current();
      if (event.key !== "Tab") return;
      const items = Array.from(
        element?.querySelectorAll<HTMLElement>(
          'button:not(:disabled),input:not(:disabled),select:not(:disabled),[tabindex="0"]',
        ) ?? [],
      );
      const first = items[0],
        last = items[items.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last?.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first?.focus();
      }
    };
    document.addEventListener("keydown", handler);
    return () => {
      document.removeEventListener("keydown", handler);
      document.body.style.overflow = originalOverflow;
      previous?.focus();
    };
  }, []);
  return (
    <div
      className="modal-backdrop"
      onMouseDown={(event) => {
        if (event.target === event.currentTarget) onClose();
      }}
    >
      <div
        ref={ref}
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-labelledby="modal-title"
      >
        <div className="modal-header">
          <div className="modal-title" id="modal-title">
            {title}
          </div>
          <Button className="icon-button" onClick={onClose} aria-label="Cerrar">
            <Icon name="close" />
          </Button>
        </div>
        {children}
      </div>
    </div>
  );
}

export default Brand;
