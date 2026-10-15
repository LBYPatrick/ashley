# Vue + TypeScript Technology Stack

Default for a new web frontend when the user has not specified a framework. Honor explicit choices and existing project conventions; the React alternative remains in `components/stack/typescript.md`.

- **Framework:** Vue + TypeScript, using single-file `.vue` components with Composition API and `<script setup lang="ts">`.
- **Toolchain:** Follow Web Frontend Tooling for dependency, build, lint, and format commands. Keep the Vue plugin and Vue TypeScript template.
- **State:** Vue reactivity for local state; Pinia when shared application state is needed.
- **Routing:** Vue Router when routing is needed.
- **Linter:** Oxlint for script blocks. Keep ESLint with `eslint-plugin-vue` and `vue-eslint-parser` only for required Vue template rules or other unsupported checks. Do not copy the React hooks/refresh ESLint template into Vue projects.
- **Type checking:** Run `vue-tsc --noEmit` in addition to the Vite build.
- **Styling:** TailwindCSS through `@tailwindcss/vite` when Tailwind is used; no separate PostCSS configuration.
- **UI behavior:** Use Vue bindings, lifecycle hooks, and transitions. React hooks, React Router, Redux, and React-specific animation/i18n examples apply only to the React alternative.
