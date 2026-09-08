# General Coding Guidelines

Follow these principles strictly:

## Language & Framework Selection
Honor explicit user language/framework choices. For existing projects, follow the established stack unless the user requests a change. For a new backend with no language specified, decide in this order:

1. **Rust** when the software runs on or close to an edge device and requires extreme performance.
2. **Python** for machine learning or data analytics when the user can tolerate lower runtime performance. Do not assume that tolerance when performance requirements are strict or unknown.
3. **Go** for all other backend cases, including performance-sensitive ML/data services that do not meet the Rust condition.

For a new web frontend with no framework specified, use **Vue + TypeScript + Vite**. A request for TypeScript alone does not imply React. Use React + TypeScript when explicitly requested or already established in the project. Keep the Vue and React stack references available, and apply only the matching framework guidance.

## SOLID & Clean Code
- **SRP:** Each class/module/function has one reason to change. Split multi-purpose functions.
- **Open/Closed:** Open for extension, closed for modification. Prefer composition and interfaces.
- **Liskov:** Subtypes substitutable for base types without altering correctness.
- **Interface Segregation:** Many small focused interfaces over one large general-purpose one.
- **Dependency Inversion:** Depend on abstractions, not concretions. Inject dependencies.

## Functional Purity
Write functions as pure as possible (same inputs → same outputs, no side effects). Isolate side effects (I/O, network, DB) at system edges. Core logic stays pure.

## Async & Concurrency
Use asyncio/multithreading/multiprocessing for I/O and CPU-bound work. Python: prefer `asyncio` + `uvloop`, use `AsyncUtil` if available. TS/JS: `Promise.all`, `Promise.allSettled`, async/await.
## Testing
After writing code, always verify: use project's test framework (pytest, jest, vitest), write minimal test scripts if none exists, or at minimum run a build. Test happy path + at least one edge case.

## YAGNI (You Aren't Gonna Need It)
Do not build features, abstractions, or infrastructure "just in case." Only implement what is required right now. If a future need arises, implement it then — the cost of adding later is almost always less than the cost of maintaining unused code. Delete dead code immediately; do not comment it out.

## DRY (Don't Repeat Yourself)
Every piece of knowledge should have a single, authoritative representation. When you see the same logic, constant, or decision expressed in more than one place, extract it into a shared function, constant, or module. But do not over-abstract — two similar-but-distinct pieces of code are not necessarily duplication. Only extract when the duplicated logic truly has one reason to change.

## Style
Follow Google's Style Guide for the language, unless the codebase has an established style — match it. Run the formatter when done.
