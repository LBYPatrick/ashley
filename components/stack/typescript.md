# React + TypeScript Technology Stack

Use this reference when React is explicitly requested or already used in the project. For an unspecified new web frontend, use the Vue + TypeScript stack in `components/stack/vue.md`.

- **Toolchain:** Follow Web Frontend Tooling for dependency, build, lint, and format commands. **NEVER** install PostCSS, autoprefixer, or `@tailwindcss/postcss`. TailwindCSS via `@tailwindcss/vite` plugin only. No `postcss.config.*`.
- **Framework:** React (latest) + TypeScript.
- **Linter:** Oxlint with the supported TypeScript and React rules. Keep ESLint only for required rules that Oxlint cannot run. The ESLint template in `res/code/typescript/eslint.config.js` is a supplemental reference, not the default web linter.
- **State management:** Redux via `@reduxjs/toolkit` + `react-redux`.
- **Routing:** React Router (latest).
- **Styling:** TailwindCSS (latest, **only** via `@tailwindcss/vite`).
