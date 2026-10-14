import assert from "node:assert/strict";
import path from "node:path";
import { fileURLToPath } from "node:url";
import { isFileLoadingAllowed, resolveConfig } from "vite";

const frontend = path.dirname(fileURLToPath(import.meta.url));
const root = path.dirname(frontend);
delete process.env.FIGMA_DEV_SERVER_HOST;
const config = await resolveConfig({ root: frontend, configFile: path.join(frontend, "vite.config.ts"), configLoader: "runner" }, "serve");
assert.equal(config.server.host, "127.0.0.1");
assert.equal(config.server.fs.strict, true);
for (const relative of ["private.xlsx", "deploy/.env", "backend/internal/orden/config.go", "AGENTS.md", "frontend/.env", "frontend/public/private.xlsx", "frontend/public/client_secret.json", "frontend/public/private.pem"]) {
  assert.equal(isFileLoadingAllowed(config, path.join(root, relative)), false, relative);
}
assert.equal(isFileLoadingAllowed(config, path.join(frontend, "src/main.tsx")), true);
assert.equal(isFileLoadingAllowed(config, path.join(root, ".figma/make/site.json")), true);
console.log("PASS: Vite listens locally, permits frontend/site metadata only, and denies financial documents, secrets, keys and backend files.");
