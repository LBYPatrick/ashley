# TypeScript Formatter & Linter Setup

## Tools
- **Prettier** — code formatter with Tailwind and import organization plugins.
- **ESLint v9+** — linter with TypeScript type-checked rules.

## Installation

Create `scripts/install-formatter.sh`: check for `pnpm` (auto-install via `npm install -g pnpm` if missing), then:
```bash
pnpm add -D prettier prettier-plugin-organize-imports prettier-plugin-tailwindcss \
  eslint @eslint/js typescript-eslint eslint-plugin-erasable-syntax-only globals
```

Then add the plugins for the project's framework:

- **Vue (default for new frontends):** `pnpm add -D eslint-plugin-vue vue-eslint-parser vue-tsc`.
- **React:** `pnpm add -D eslint-plugin-react-hooks eslint-plugin-react-refresh`.
- For non-UI TypeScript projects, omit framework plugins.

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
**React only:** Reference template in `res/code/typescript/eslint.config.js`. ESLint v9 flat config with `typescript-eslint` (type-checked), React hooks/refresh plugins, erasable syntax plugin, `_`-prefixed unused vars ignored. Adjust `tsconfigRootDir` and `project` paths.

**Vue:** Use the Vue plugin's flat recommended configuration with TypeScript rules. Keep `vue-eslint-parser` as the parser for `.vue` files and configure its script parser for TypeScript. Include `.vue` files in lint/type checks; do not enable React hooks/refresh rules. Preserve an existing compatible configuration.

## tidy.sh

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
