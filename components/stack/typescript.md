# TypeScript / JavaScript Technology Stack

Use this stack unless the user specifies otherwise.

- **Package management:** pnpm.
- **Build tool:** Vite. **NEVER** install PostCSS, autoprefixer, or `@tailwindcss/postcss`. TailwindCSS via `@tailwindcss/vite` plugin only. No `postcss.config.*`.
- **Framework:** React (latest) + TypeScript.
- **Formatter/linter:** Prettier + ESLint v9+ (`.js` config, not `.mjs`). Ref configs in `res/code/typescript/eslint.config.js` and `.prettierrc`. Key plugins: `typescript-eslint` (type-checked), `eslint-plugin-react-hooks`, `eslint-plugin-react-refresh`, `eslint-plugin-erasable-syntax-only`.
- **State management:** Redux via `@reduxjs/toolkit` + `react-redux`.
- **Routing:** React Router (latest).
- **Styling:** TailwindCSS (latest, **only** via `@tailwindcss/vite`).
