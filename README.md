# Orden

Finanzas personales privadas: React + TypeScript, Go y PostgreSQL. Un único
propietario configurado mediante `OWNER_EMAIL`; sin registro público y sin
correos personales o secretos en el código.

## Producción: Ubuntu ARM64 + Coolify

**Guía paso a paso y variables:** [deploy/README.md](deploy/README.md).

1. En Coolify crea una aplicación desde este repositorio, rama `main`.
2. Selecciona **Docker Compose**, base `/`, archivo `/docker-compose.yml`.
3. Configura las variables de ejecución del panel; usa `.env.example` solo como
   referencia, nunca subas un `.env` real.
4. Asigna el dominio HTTPS únicamente al servicio **web**, puerto interno **8080**.
5. Despliega y genera el código de activación desde la terminal del backend.

El frontend de producción es estático, servido por Nginx no root. No contiene
Node, Vite, fuentes TypeScript ni secretos. PostgreSQL y la API no publican
puertos. TLS lo termina el proxy de Coolify, no este contenedor.

## Funcionalidad

- Cuentas, gastos, ingresos y aportes de inversión persistentes.
- CLP sin fracciones y USD con dos decimales; monedas separadas, sin conversión.
- Filtros de mes/categoría/concepto, paginación y gráficos con datos reales.
- Activación y recuperación mediante códigos de consola de un uso (15 minutos).
- Gmail OAuth opcional para compras MACH compatibles; Bci pendiente de formato.
- Clasificación Naive Bayes local en Go, entrenada solo con gastos confirmados.

Saldo = saldo inicial + ingresos − gastos − aportes de inversión. No se inventan
precios de inversiones, rendimientos ni patrimonio; no hay transferencias,
presupuestos o cotizaciones automáticas.

## Ramas y pruebas

`main` es la rama de producción. **Todo cambio posterior debe hacerse en otra
rama**, pasar CI y entrar mediante pull request. Activa también las reglas de
protección remotas indicadas en la guía. Para habilitar el bloqueo local:

```sh
git config core.hooksPath .githooks
git switch -c feature/descripcion
```

Desarrollo solo desde esa rama: `pnpm install`, `pnpm dev`; Vite escucha en
`127.0.0.1`, limita su filesystem al frontend y deniega documentos financieros.
El backend usa `backend/.env.example`; HTTP de desarrollo solo acepta origen local.
Vite no se utiliza para publicar producción. La previsualización Figma puede
configurar su host explícitamente sin ampliar el acceso a archivos personales.

```sh
pnpm typecheck
pnpm build
pnpm audit
python3 scripts/check-public-repo.py --history
cd backend
go test -race ./...
go vet ./...
```

Las pruebas de integración requieren `TEST_DATABASE_URL`, crean un esquema
aleatorio y lo eliminan: **usa una base desechable, nunca producción**.

## Seguridad

Argon2id, sesiones hash/HttpOnly/Secure/SameSite, CSRF y Origin, SQL parametrizado,
límites por cliente y proxy interno autenticado. Gmail usa PKCE y AES-GCM; no se
guardan cuerpos de correo ni comercios. F12 no se puede bloquear: la privacidad
depende del servidor y de proteger tus dispositivos, Coolify, Cloudflare y SSH.

Los documentos financieros, backups y secretos están ignorados y excluidos de
Docker; CI añade una revisión de archivos públicos e historial de blobs. Ningún
scanner demuestra ausencia absoluta de secretos. Estado de revisión: [AUDITORIA.md](AUDITORIA.md).
