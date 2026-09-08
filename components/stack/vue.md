# Vue + TypeScript Technology Stack

Default for a new web frontend when the user has not specified a framework. Honor explicit choices and existing project conventions; the React alternative remains in `components/stack/typescript.md`.

- **Framework:** Vue + TypeScript, using single-file `.vue` components with Composition API and `<script setup lang="ts">`.
- **Build tool:** Vite with its Vue plugin. Use the Vue TypeScript template.
- **Package management:** pnpm.
- **State:** Vue reactivity for local state; Pinia when shared application state is needed.
- **Routing:** Vue Router when routing is needed.
- **Formatter/linter:** Prettier and ESLint with Vue and TypeScript rules. Parse `.vue` files with `vue-eslint-parser` and TypeScript script blocks with the TypeScript parser. Do not copy the React hooks/refresh ESLint template into Vue projects.
- **Type checking:** Run `vue-tsc --noEmit` in addition to the Vite build.
- **Styling:** TailwindCSS through `@tailwindcss/vite` when Tailwind is used; no separate PostCSS configuration.
- **UI behavior:** Use Vue bindings, lifecycle hooks, and transitions. React hooks, React Router, Redux, and React-specific animation/i18n examples apply only to the React alternative.
