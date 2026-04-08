# Google Style Guide Compliance

Follow Google's Style Guide for the project's language. Key rules by language:

## Python
Reference: [Google Python Style Guide](https://google.github.io/styleguide/pyguide.html)
- **Naming:** `module_name`, `function_name`, `ClassName`, `CONSTANT_NAME`, `_private`
- **Imports:** One per line, grouped: stdlib → third-party → local. Absolute imports preferred.
- **Docstrings:** Google-style (`Args:`, `Returns:`, `Raises:` sections). Required for public APIs.
- **Type hints:** Use for all function signatures. Use `|` union syntax (3.10+).
- **Line length:** 80 characters. Exceptions: URLs, imports, comments with long paths.

## TypeScript / JavaScript
Reference: [Google TypeScript Style Guide](https://google.github.io/styleguide/tsguide.html)
- **Naming:** `camelCase` functions/variables, `PascalCase` classes/interfaces/types/enums, `UPPER_SNAKE` constants.
- **Imports:** Named imports preferred. No wildcard (`*`) imports. Use `type` imports for type-only.
- **Exports:** Named exports preferred over default exports.
- **Nullability:** Use `undefined` over `null` where possible. Explicit `null` checks, no loose equality (`==`).
- **Formatting:** 2-space indent, semicolons required, single quotes.

## Go
Reference: [Google Go Style Guide](https://google.github.io/styleguide/go/)
- **Naming:** `MixedCaps` / `mixedCaps` (no underscores). Short variable names in small scopes.
- **Errors:** Return errors, don't panic. Wrap with `fmt.Errorf("...: %w", err)`.
- **Formatting:** `gofmt` is authoritative. No style debates.
- **Comments:** Package comment on the `package` line. Exported symbols documented with `// FuncName ...`.

## Java
Reference: [Google Java Style Guide](https://google.github.io/styleguide/javaguide.html)
- **Naming:** `ClassName`, `methodName`, `CONSTANT_NAME`, `parameterName`.
- **Imports:** No wildcard imports. Static imports separate block.
- **Braces:** K&R style (opening brace on same line). Required even for single-statement bodies.
- **Column limit:** 100 characters.

## C++
Reference: [Google C++ Style Guide](https://google.github.io/styleguide/cppguide.html)
- **Naming:** `ClassName`, `function_name`, `variable_name`, `kConstantName`, `MACRO_NAME`.
- **Headers:** `#pragma once` or include guards. Forward-declare when possible.
- **Ownership:** Use `std::unique_ptr` for ownership, raw pointers for non-owning references.
- **Formatting:** 2-space indent, 80-character lines.

## Shell (Bash)
Reference: [Google Shell Style Guide](https://google.github.io/styleguide/shellguide.html)
- **Shebang:** `#!/bin/bash`. Use `set -euo pipefail`.
- **Naming:** `lower_snake_case` for variables and functions. `UPPER_SNAKE` for constants/env vars.
- **Quoting:** Always double-quote variables: `"$var"`, `"${array[@]}"`.
- **Functions:** Use `my_func()` syntax (no `function` keyword). Document with a comment above.

## Dart / Flutter
Reference: [Effective Dart: Style](https://dart.dev/effective-dart/style)
- **Naming:** `UpperCamelCase` types, `lowerCamelCase` members/variables, `lowercase_with_underscores` packages/files.
- **Imports:** `dart:` first, then `package:`, then relative. Alphabetized within groups.
- **Formatting:** `dart format` is authoritative. Trailing commas for multi-line collections.
- **Documentation:** `///` doc comments for public APIs. First sentence as summary.

## General Rules (All Languages)
- Run the project's configured formatter before considering style complete.
- When the project has an existing style that conflicts with Google's guide, **match the project's existing style** for consistency.
- These guides cover the most common rules. For edge cases, consult the full guide linked above.
