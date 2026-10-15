# Web Frontend Tooling

Apply these rules to JavaScript and TypeScript web frontends. Keep the selected framework. Other languages and native mobile applications keep their own tooling.

## Toolchain Choice

1. Follow explicit user choices. Keep a working project toolchain during unrelated feature changes.
2. For a new web frontend or requested toolchain setup, try **Vite+** first. Use its `vp` CLI.
3. Check platform, framework, plugin, and deployment requirements before setup. If Vite+ cannot meet them, use **pnpm** with the compatible build tool. Report the reason for the fallback.
4. Keep the project's package manager declaration and lockfile. Vite+ uses the underlying package manager; do not create competing lockfiles.

Use the [Vite+ setup guide](https://www.viteplus.dev/guide/) and the installed CLI's help for current setup commands. A global CLI installation is optional. A project-local CLI can run through `pnpm exec vp` when the project uses pnpm.

## Lint and Format

- Use **Oxlint** as the primary linter for web frontends, with either toolchain choice.
- With Vite+, use `vp lint` or `vp check`. Keep lint and format settings in the root `vite.config.ts`.
- With the pnpm fallback, install `oxlint` as a development dependency. Run `pnpm exec oxlint .` in CI and `pnpm exec oxlint --fix .` for local fixes.
- Use Oxfmt through Vite+ when it supports the required formatting behavior. Use Prettier when required by the project or formatter plugins.
- Keep ESLint only for required rules that Oxlint cannot run. Disable duplicate rules when both linters are needed.
- Oxlint checks script blocks in Vue files. Keep Vue template checks when required. Run `vue-tsc --noEmit` for Vue and the configured TypeScript check for other frameworks.

## Project Commands

| Task | Vite+ | pnpm fallback |
|------|-------|---------------|
| Install dependencies | `vp install` | `pnpm install` |
| Add a dependency | `vp add <package>` | `pnpm add <package>` |
| Run configured development tasks | `vp run dev` | `pnpm run dev` |
| Check formatting and lint | `vp check` | `pnpm exec prettier --check .` and `pnpm exec oxlint .` |
| Apply format and lint fixes | `vp check --fix` | `pnpm exec prettier --write .` and `pnpm exec oxlint --fix .` |
| Run configured test tasks | `vp run test` | `pnpm run test` |
| Run configured build tasks | `vp run build` | `pnpm run build` |

Use `vp dev`, `vp test`, and `vp build` for the integrated tools. Use `vp run <task>` for project scripts. Keep required framework checks in those scripts or Makefile targets.
