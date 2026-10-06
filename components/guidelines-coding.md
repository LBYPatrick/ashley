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

## Backend Architecture: Domain Driven Design
Use Domain Driven Design (DDD) for backend servers in every language, including Go. Organize files by bounded context and business capability, then separate domain rules, application use cases, and infrastructure/transport adapters within each context. This architecture requirement takes precedence over Go layout advice that suggests a different organization.

For example, a Go service can use `cmd/server/main.go` for composition and `internal/orders/{domain,application,infrastructure,transport}/` for the orders context. Name files after business concepts (`order.go`, `place_order.go`) and keep tests beside the code. Domain code must not import HTTP frameworks, database drivers, or infrastructure packages; adapters depend inward through small interfaces owned by their consumers. Keep transaction orchestration in application services and business invariants in domain types. Add aggregates, repositories, and domain events only when the business model needs them; do not create empty layers or generic base repositories. For existing servers, apply these boundaries to touched functionality without an unrelated wholesale rewrite.

## Test Scripts & One-Shot Scripts
Prefer **Go** for new standalone verification scripts, test harnesses, and one-shot automation, including in non-Go repositories. Use the standard library where practical and run small programs with `go run path/to/script.go`; use `go run ./scripts/task` for a multi-file command within a module. Prefer Go over Python unless the task needs Python-specific libraries, an existing script is being maintained, or the available environment cannot run Go. Keep native unit tests in the project's existing test framework; this preference does not replace pytest, Jest, or other established suites.

## Functional Purity
Write functions as pure as possible (same inputs → same outputs, no side effects). Isolate side effects (I/O, network, DB) at system edges. Core logic stays pure.

## Async & Concurrency
Use asyncio/multithreading/multiprocessing for I/O and CPU-bound work. Python: prefer `asyncio` + `uvloop`, use `AsyncUtil` if available. TS/JS: `Promise.all`, `Promise.allSettled`, async/await.
Go: use bounded goroutines for independent work, pass `context.Context` for cancellation and deadlines, and give each goroutine an owner responsible for its lifetime. Synchronize shared state; do not introduce concurrency without a clear benefit.

## Testing
After writing code, always verify: use the project's test framework (Go `testing`, pytest, jest, vitest), write minimal Go test scripts if none exists, or at minimum run a build. Test happy path + at least one edge case. In Go, use `*_test.go`, `TestXxx(t *testing.T)`, table-driven cases when useful, and `t.Cleanup` for resources. Run `go test ./...` and use `go test -race ./...` when supported for concurrent code.

## YAGNI (You Aren't Gonna Need It)
Do not build features, abstractions, or infrastructure "just in case." Only implement what is required right now. If a future need arises, implement it then — the cost of adding later is almost always less than the cost of maintaining unused code. Delete dead code immediately; do not comment it out.

## DRY (Don't Repeat Yourself)
Every piece of knowledge should have a single, authoritative representation. When you see the same logic, constant, or decision expressed in more than one place, extract it into a shared function, constant, or module. But do not over-abstract — two similar-but-distinct pieces of code are not necessarily duplication. Only extract when the duplicated logic truly has one reason to change.

## Style
Follow Google's Style Guide for the language, unless the codebase has an established style — match it. Run the formatter when done.
