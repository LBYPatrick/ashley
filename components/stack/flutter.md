# Flutter / Dart Technology Stack

Use this stack unless the user specifies otherwise.

## Core Setup
- **Package management:** `flutter pub`, `pubspec.yaml` as canonical config.
- **Targets:** Linux and Web by default. Guard platform-specific APIs (`Platform` checks / `kIsWeb`).
- **State management:** Riverpod (`flutter_riverpod` + `riverpod_annotation` + `riverpod_generator` + `build_runner`). Prefer `@riverpod` annotated providers. One concern per provider.
- **Routing:** GoRouter (`go_router`). Centralized `router.dart`, `ShellRoute` for persistent layouts, typed route params.
- **Networking:** dio with shared instance (base URL, interceptors for auth/logging/errors, timeouts). Models via `json_serializable` or `freezed`.

## Animations
- Built-in framework: `AnimationController`, `Tween`, `AnimatedBuilder`.
- Simple: `AnimatedContainer`, `AnimatedOpacity`, `AnimatedSwitcher`.
- Route transitions: `Hero` widgets + `PageRouteBuilder` with custom `transitionsBuilder`.
- Durations: fast=100ms, normal=300ms, slow=500ms.
- Curves: `easeOut` (entrances), `easeIn` (exits), `easeInOut` (symmetric).

## Styling & Theming
- Material 3 (`useMaterial3: true`). Centralized `AppTheme` with light/dark `ThemeData`.
- `ColorScheme.fromSeed()` for dynamic color. `ThemeMode.system` for auto dark mode.
- Use `Theme.of(context).colorScheme` / `.textTheme` — never hardcode colors/sizes.
- Subtle tonal fills (`surfaceContainerLow`/`High`) instead of card shadows. `BorderRadius.circular(12)`.

## Code Generation & Quality
- `build_runner`: `dart run build_runner build --delete-conflicting-outputs`. Generators: `riverpod_generator`, `json_serializable`, `freezed`. Files: `.g.dart` / `.freezed.dart` with `part` directives.
- Formatter: `dart format`. Linter: `flutter analyze`.
- `analysis_options.yaml`: `include: package:flutter_lints/flutter.yaml` with rules: `prefer_const_constructors`, `prefer_const_declarations`, `avoid_print`, `prefer_single_quotes`, `sort_constructors_first`, `unawaited_futures`.

## Project Structure
```
lib/
  main.dart, router.dart, theme.dart
  features/<feature>/ → presentation/, providers/, data/, models/
  shared/ → widgets/, providers/, models/, utils/
```

## Testing
- `flutter_test` for widget/unit tests. `mocktail` for mocking. `ProviderContainer` for isolated provider tests.
