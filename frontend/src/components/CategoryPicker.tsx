import { useId, useRef, useState } from "react";
import { Button, Icon, type IconName } from "./ui";

const normalize = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase();
const styles: Record<string, { icon: IconName; color: string }> = {
  "Vida diaria": { icon: "food", color: "#b3ca85" },
  Transporte: { icon: "transport", color: "#83b9db" },
  Entretenimiento: { icon: "play", color: "#b69adc" },
  Vacaciones: { icon: "sun", color: "#e1be7b" },
  Ocio: { icon: "play", color: "#d2a1b8" },
  Personal: { icon: "user", color: "#8dc7b9" },
  Préstamos: { icon: "wallet", color: "#b5acdc" },
  "Pagos varios": { icon: "gift", color: "#d6a485" },
};

export default function CategoryPicker({
  value,
  categories,
  loading,
  error,
  onRetry,
  onChange,
  mode = "select",
}: {
  value: string;
  categories: { id: number; name: string }[];
  loading: boolean;
  error: boolean;
  onRetry: () => void;
  onChange: (category: string) => void;
  mode?: "select" | "filter";
}) {
  const id = useId();
  const trigger = useRef<HTMLButtonElement>(null);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState("");
  const [active, setActive] = useState("");
  const groups = new Map<string, { id: number; name: string; label: string }[]>();
  for (const item of categories) {
    const [group, ...rest] = item.name.split(" > ");
    const entries = groups.get(group) ?? [];
    entries.push({ ...item, label: rest.join(" > ") || group });
    groups.set(group, entries);
  }
  const selectedGroup = value.split(" > ")[0];
  const selectedLabel = value.split(" > ").slice(1).join(" > ");
  const selectedStyle = styles[selectedGroup] ?? { icon: "tag" as const, color: "#b3ca85" };
  const query = normalize(search.trim());
  const visible = [...groups].filter(([, items]) =>
    items.some((item) => normalize(item.name).includes(query)),
  );
  const choose = (name: string) => {
    onChange(name);
    setOpen(false);
    setSearch("");
    trigger.current?.focus();
  };
  return (
    <div
      className={`category-picker field full ${mode === "filter" ? "category-filter" : ""}`}
      onKeyDown={(event) => {
        if (event.key === "Escape" && open) {
          event.stopPropagation();
          setOpen(false);
          trigger.current?.focus();
        }
      }}
    >
      <span id={`${id}-label`}>{mode === "filter" ? "Filtrar por categoría" : "Categoría"}</span>
      <button
        type="button"
        ref={trigger}
        className={`category-trigger ${value ? "has-value" : ""}`}
        aria-labelledby={`${id}-label ${id}-value`}
        aria-expanded={open}
        aria-controls={open ? `${id}-panel` : undefined}
        onClick={() => {
          setOpen(!open);
          setActive(selectedGroup);
          setSearch("");
        }}
      >
        <span className="category-symbol" style={{ color: selectedStyle.color }}>
          <Icon name={selectedStyle.icon} size={19} />
        </span>
        <span id={`${id}-value`} className="category-trigger-text">
          <strong>
            {value
              ? selectedGroup
              : mode === "filter"
                ? "Todas las categorías"
                : "Elige una categoría"}
          </strong>
          <small>
            {selectedLabel ||
              (value
                ? "Categoría seleccionada"
                : mode === "filter"
                  ? "Selecciona para filtrar tus gastos"
                  : "Explora por grupo o busca una subcategoría")}
          </small>
        </span>
        <span className={open ? "category-chevron is-open" : "category-chevron"}>
          <Icon name="chevron" size={18} />
        </span>
      </button>
      {open && (
        <div
          id={`${id}-panel`}
          className="category-panel"
          role="region"
          aria-label="Elegir categoría"
        >
          <label className="category-search">
            <Icon name="search" size={17} />
            <input
              autoFocus
              aria-label="Buscar categorías"
              placeholder="Buscar comida, metro, regalos…"
              value={search}
              onChange={(event) => setSearch(event.target.value)}
            />
          </label>
          {loading ? (
            <p className="category-empty" role="status">
              Cargando tus categorías…
            </p>
          ) : error ? (
            <p className="category-empty" role="alert">
              No pudimos cargar las categorías.{" "}
              <Button className="text-button" onClick={onRetry}>
                Reintentar
              </Button>
            </p>
          ) : (
            <div className="category-browser">
              {!visible.length && (
                <p className="category-empty">
                  {query
                    ? "No hay categorías que coincidan."
                    : "Crea tus categorías en Conexiones."}
                </p>
              )}
              <div className="category-groups">
                {visible.map(([group, items]) => {
                  const appearance = styles[group] ?? { icon: "tag" as const, color: "#b3ca85" };
                  const standalone = items.length === 1 && items[0].name === group;
                  return (
                    <Button
                      key={group}
                      className={`category-group ${active === group || selectedGroup === group ? "is-active" : ""}`}
                      aria-expanded={standalone ? undefined : active === group}
                      onClick={() =>
                        standalone
                          ? choose(items[0].name)
                          : setActive(active === group ? "" : group)
                      }
                    >
                      <span className="category-symbol" style={{ color: appearance.color }}>
                        <Icon name={appearance.icon} size={19} />
                      </span>
                      <strong>{group}</strong>
                      <small>{standalone ? "Seleccionar" : `${items.length} opciones`}</small>
                    </Button>
                  );
                })}
              </div>
              {(query ? visible : visible.filter(([group]) => group === active)).map(
                ([group, items]) => (
                  <div className="category-options" key={group}>
                    <span>{group}</span>
                    <div>
                      {items
                        .filter((item) => normalize(item.name).includes(query))
                        .map((item) => (
                          <Button
                            key={item.id}
                            className={`category-option ${value === item.name ? "is-selected" : ""}`}
                            aria-pressed={value === item.name}
                            onClick={() => choose(item.name)}
                          >
                            {item.label}
                            {value === item.name && <Icon name="check" size={14} />}
                          </Button>
                        ))}
                    </div>
                  </div>
                ),
              )}
            </div>
          )}
          <div className="category-panel-footer">
            <span>
              {mode === "filter" ? "Filtra sin escribir el nombre" : "Organizadas a tu manera"}
            </span>
            <Button className="text-button" onClick={() => choose("")}>
              {mode === "filter" ? "Todas las categorías" : "Dejar pendiente"}
            </Button>
          </div>
        </div>
      )}
      {mode === "filter" && value && !open && (
        <Button className="text-button category-filter-reset" onClick={() => choose("")}>
          Quitar filtro
        </Button>
      )}
    </div>
  );
}
