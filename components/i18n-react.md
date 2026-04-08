# React i18n with i18next

## Dependencies
```bash
pnpm add i18next react-i18next i18next-browser-languagedetector
```

## Default Locales
- `en` — English (US), `zh` / `zh-CN` — Simplified Chinese. Add more only if user requests.

## Architecture

### Language Files
Store as JSON in `res/` (or `public/locales/`): `en.lang.json`, `zh.lang.json`, etc. Nested by feature:
```json
{ "global": { "name": "App Name" }, "shared": { "btn_confirm": "Confirm", "btn_cancel": "Cancel" }, "login": { "prompt_enter_name": "Enter your name" } }
```

### LocaleHelper
Ref: `res/code/typescript/LocaleHelper.ts`. Flattens nested JSON into `/`-separated keys (e.g., `shared/btn_confirm`). Maps locale variants (`zh-CN`→`zh`). Returns i18next `InitOptions` with `fallbackLng: "en"`, browser detection, localStorage/sessionStorage caching. Copy to `src/misc/`, adjust import paths.

### Initialization
Ref: `res/code/typescript/Locale.tsx`. Create `src/components/Locale.tsx`:
```typescript
import i18n from "i18next";
import { initReactI18next } from "react-i18next";
import LanguageDetector from "i18next-browser-languagedetector";
import LocaleHelper from "../misc/LocaleHelper";
i18n.use(LanguageDetector).use(initReactI18next).init(LocaleHelper.getConfig());
export default i18n;
```
Import in `main.tsx`: `import "./components/Locale";`

### Usage
```tsx
import { useTranslation } from "react-i18next";
function MyComponent() { const { t } = useTranslation(); return <button>{t("shared/btn_confirm")}</button>; }
```

## Hardcoded String Rules
**EVERY** user-visible string must use `t()`: JSX text, attributes (placeholder, title, aria-label, alt), button/link labels, error messages in UI, validation messages, table headers, nav labels, dialog titles/body, tooltips, empty/loading states, notifications.
**Exceptions:** console.log/error, CSS classes, URLs, technical identifiers, numbers, punctuation-only.

## Exhaustive Scanning
Read EVERY `.tsx`/`.ts` file in `src/`. Do not rely on grep alone. After converting each file, re-read to verify no hardcoded user-facing strings remain.

## Syncing Keys
After conversion: (1) scan all `.tsx`/`.ts` for `t("...")` calls, (2) collect all keys, (3) compare against each language JSON, (4) add missing keys with translations, (5) remove orphaned keys, (6) verify all locales have identical key sets.
