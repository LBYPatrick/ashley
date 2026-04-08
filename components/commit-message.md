# Commitizen-Style Commit Messages

Format every commit message using the [Conventional Commits](https://www.conventionalcommits.org/) spec (commitizen style):

```
<type>(<scope>): <short summary>

<body>
```

## Rules

1. **type** — one of: `feat`, `fix`, `docs`, `style`, `refactor`, `perf`, `test`, `build`, `ci`, `chore`, `revert`.
2. **scope** — optional, lowercase name of the affected module, component, or area (e.g., `auth`, `api`, `cli`).
3. **short summary** — imperative mood, lowercase, no period, max 72 characters.
4. **body** — optional, separated from the summary by a blank line. Explain *what* changed and *why*. Wrap at 72 characters.

## Choosing the Type

| Type | Use when … |
|---|---|
| `feat` | Adding new functionality visible to users |
| `fix` | Correcting a bug |
| `docs` | Documentation-only changes |
| `style` | Formatting, whitespace, semicolons — no logic change |
| `refactor` | Code restructuring without changing behavior |
| `perf` | Performance improvement |
| `test` | Adding or updating tests |
| `build` | Build system or dependency changes |
| `ci` | CI/CD pipeline changes |
| `chore` | Maintenance tasks (tooling, config, release prep) |
| `revert` | Reverting a previous commit |

## Examples

```
feat(auth): add OAuth2 login flow

Integrate Google and GitHub OAuth2 providers.
Tokens are stored in httpOnly cookies with 7-day expiry.
```

```
fix(api): prevent duplicate webhook delivery

Race condition in the queue consumer caused the same
event to be dispatched twice when the worker restarted
mid-batch.
```

```
chore: bump dependencies to latest patch versions
```
