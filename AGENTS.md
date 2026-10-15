# Orden

React + Vite + Tailwind CSS project running inside Figma Make.

## Development Server

`main` is the production branch. This production bootstrap is explicitly
authorized on main; all subsequent changes must use a feature/release branch
and a reviewed pull request. Never commit/push directly to main or deploy
without explicit authorization. Preserve real data and production volumes.

A Vite development server is **already running** on `$PORT` (default 8443). You don't need to start it manually.

- Preview URL: The user can access the running app through the preview panel
- Hot reload: Changes to source files are reflected immediately

## Project Structure

This is the canonical project structure. Start with task-relevant files below. Only follow imports or inspect other files when required, when a documented path is missing, or when the repository contradicts this guide.

- `frontend/src/main.tsx` - React entrypoint; imports `frontend/src/index.css` and mounts `frontend/src/App.tsx`
- `frontend/src/App.tsx` - Primary application component and the usual starting point for UI work
- `frontend/src/index.css` - Global CSS entrypoint and Tailwind CSS v4 import
- `frontend/index.html` - Vite HTML shell loading `/src/main.tsx` relative to the frontend root
- `frontend/package.json` - Frontend dependencies and development scripts
- `frontend/vite.config.ts` - Vite configuration with React, Tailwind CSS v4, and Figma Make plugins plus the `@` alias for frontend `src`
- `backend/cmd/orden/main.go` - Go server and server-only activation/recovery CLI
- `backend/internal/orden/` - pgx persistence, authentication, security middleware, financial API and tests
- `backend/internal/orden/migrations/` - Embedded versioned PostgreSQL migrations
- `backend/internal/orden/gmail.go` and `mail_parser.go` - Optional encrypted Gmail OAuth, bounded polling and purchase-only parsing
- `backend/internal/orden/classifier.go` - Lightweight Naive Bayes, editable categories and confirmed training examples
- `frontend/src/components/Settings.tsx` - Optional Gmail connection, card-to-account rules and category management
- `frontend/src/components/Calculator.tsx` and `frontend/src/lib/calculator.ts` - Private saved planning configuration and exact minor-unit simulation; never creates financial movements
- `frontend/src/lib/navigation.ts` - SPA paths, including OAuth return; keep deep links, back/forward and reload working
- `backend/internal/orden/calculator.go` - Authenticated, CSRF-protected planning preferences per owner/currency
- `frontend/tests/` - Synthetic calculation/navigation unit tests and Playwright browser checks; never contact production APIs
- `docker-compose.yml` - Production-only Coolify stack, ARM64/AMD64, no published host ports
- `frontend/Dockerfile` - Static frontend build and non-root Nginx runtime (no Vite in production)
- `deploy/README.md` - Coolify runtime variables, ingress, activation, recovery and operations
- `scripts/check-public-repo.py` - Redacted public-file/history privacy checks
- `package.json` and `pnpm-workspace.yaml` - Root workspace and forwarding scripts
- `vite.config.ts` - Compatibility entrypoint delegating to frontend configuration
- `.mise.toml` - Toolchain versions for Node.js and pnpm

## Dependencies

- Runtime: React 19 and React DOM 19
- Styling: Tailwind CSS v4 with the `@tailwindcss/vite` plugin
- Build tooling: Vite 8, TypeScript 5.7, and `@vitejs/plugin-react`
- Formatting: oxfmt

## Styling

This project uses **Tailwind CSS v4** through the `@tailwindcss/vite` plugin configured in `frontend/vite.config.ts`. `frontend/src/index.css` imports Tailwind with `@import 'tailwindcss';`. Use Tailwind utility classes directly in JSX and put global CSS or Tailwind v4 theme customization in `frontend/src/index.css`. This scaffold does not need a Tailwind config file or PostCSS config.

`frontend/src/main.tsx` imports `frontend/src/index.css`, so global font wiring belongs in `frontend/src/index.css`. Keep CSS `@import` statements first, then add any `@font-face` rules and font-family defaults there.

## Code quality

- Use double quotes for strings containing apostrophes (`"We're here to help"`), or escape them in single-quoted strings. An unescaped apostrophe in a single-quoted string breaks the build.
- Ensure JSX tags are closed and braces are balanced.
- Export components as default exports.

## Backend and security

- Use parameterized pgx queries. Money is decimal strings in JSON and numeric in PostgreSQL.
- Never add credentials, database URLs, session tokens or activation codes to frontend source or logs.
- Configure the single-owner email only with OWNER_EMAIL. SQL restricts users.id to 1; no public registration. Never embed personal email addresses in source, migrations or docs.
- Production requires HTTPS and INTERNAL_PROXY_SECRET; trust client IP only from the authenticated internal proxy. Do not expose API/PostgreSQL ports.
- All financial endpoints require a session; mutations also require matching Origin and CSRF token.
- Gmail credentials stay server-side and encrypted; never log or persist raw email bodies. Unknown bank formats must not create movements.
- Train only on explicitly confirmed expense categories, never on unconfirmed predictions. Keep inference bounded and local.
- Run `pnpm typecheck`, `pnpm build`, `go test -race ./...` and `go vet ./...` for relevant changes.
- Integration tests use `TEST_DATABASE_URL` and an isolated random schema. Never run against production.
