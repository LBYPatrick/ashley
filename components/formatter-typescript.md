# TypeScript Formatter & Linter Setup

## Web Frontends

Follow Web Frontend Tooling. Use Oxlint as the primary linter with Vite+ and with the pnpm fallback.

- **Vite+:** Use `vp check --fix` in `tidy.sh` and `vp check` in CI. Keep lint and format settings in the root `vite.config.ts`.
- **pnpm fallback:** Install `oxlint`, `prettier`, and required formatter plugins as development dependencies. Run Prettier, then `pnpm exec oxlint --fix .` in `tidy.sh`. CI must check formatting and run Oxlint without `--fix`.
- **Vue:** Keep required template checks and `vue-tsc --noEmit`. Oxlint covers script blocks, not Vue template rules.
- **React:** Enable supported React rules in Oxlint. Use the bundled ESLint reference only for required checks that Oxlint cannot run.

Do not apply the non-web ESLint setup below as the default for a web frontend.

## Non-Web TypeScript Tools

Keep the established toolchain for TypeScript projects that are not web frontends. When setting up Prettier and ESLint for those projects, use the following reference.

- **Prettier** — code formatter with Tailwind and import organization plugins.
- **ESLint v9+** — linter with TypeScript type-checked rules.

### Installation for Non-Web TypeScript

Create `scripts/install-formatter.sh`: check for `pnpm` (auto-install via `npm install -g pnpm` if missing), then:
```bash
pnpm add -D prettier prettier-plugin-organize-imports prettier-plugin-tailwindcss \
  eslint @eslint/js typescript-eslint eslint-plugin-erasable-syntax-only globals
```

Omit framework plugins for non-UI TypeScript projects.

## Configuration Files

### `.prettierrc`
```json
{
  "trailingComma": "es5",
  "printWidth": 80,
  "tabWidth": 2,
  "semi": true,
  "singleQuote": true,
  "tailwindStylesheet": "./src/index.css",
  "plugins": ["prettier-plugin-organize-imports", "prettier-plugin-tailwindcss"]
}
```

### `.prettierignore`
```
package-lock.json
pnpm-lock.yaml
node_modules
```

### `eslint.config.js`

Use this reference only for an established ESLint setup or required checks that Oxlint cannot run. Disable rules already checked by Oxlint.

**React only:** Reference template in `res/code/typescript/eslint.config.js`. ESLint v9 flat config with `typescript-eslint` (type-checked), React hooks/refresh plugins, erasable syntax plugin, `_`-prefixed unused vars ignored. Adjust `tsconfigRootDir` and `project` paths.

**Vue:** Use the Vue plugin's flat recommended configuration with TypeScript rules. Keep `vue-eslint-parser` as the parser for `.vue` files and configure its script parser for TypeScript. Include `.vue` files in lint/type checks; do not enable React hooks/refresh rules. Preserve an existing compatible configuration.

## tidy.sh for Non-Web TypeScript

Create `tidy.sh` in project root:
1. Lazy-check: if `pnpm exec prettier --version` fails, run `scripts/install-formatter.sh`. Skip with `--skip-check`.
2. `pnpm exec prettier --write "src/**/*.{ts,tsx,vue}"`
3. `pnpm exec eslint --fix .`

## Makefile Integration
```makefile
format: ## Run formatter and linter
	bash tidy.sh
tidy: format
```
