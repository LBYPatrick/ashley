import { type InitOptions, type Resource } from "i18next";

// NOTE: Update these import paths to point to your project's language JSON files.
// import en from "../../res/en.lang.json";
// import zh from "../../res/zh.lang.json";
// import vi from "../../res/vi.lang.json";
// import ja from "../../res/ja.lang.json";

type LangDict = Record<string, Record<string, unknown> | unknown>;

export default class LocaleHelper {
  /**
   * Flatten a dictionary using nested config path querying format.
   * For example, the dictionary {a: {b: {c: 1}}} will be flattened to {a/b/c: 1}
   * @param dict The dictionary to flatten
   * @returns
   */
  static getFlatObject(
    dict: Record<string, Record<string, unknown> | unknown>,
  ): Record<string, unknown> {
    const flatObject: Record<string, unknown> = {};
    Object.entries(dict).forEach(([key, value]) => {
      if (typeof value === "object" && value !== null) {
        Object.assign(
          flatObject,
          Object.fromEntries(
            Object.entries(
              this.getFlatObject(
                value as Record<string, Record<string, unknown> | unknown>,
              ),
            ).map(([k, v]) => [key + "/" + k, v]),
          ),
        );
      } else {
        flatObject[key] = value;
      }
    });
    return flatObject;
  }

  static getLocaleMapping(
    langFiles: Record<string, LangDict>,
  ): Record<string, Record<string, unknown>> {
    const mapping: Record<string, Record<string, unknown>> = {};
    for (const [locale, dict] of Object.entries(langFiles)) {
      mapping[locale] = this.getFlatObject(dict);
    }
    return mapping;
  }

  static getConfig(langFiles: Record<string, LangDict>): InitOptions {
    const resourceConfig: Resource = {};
    Object.entries(this.getLocaleMapping(langFiles)).forEach(([key, value]) => {
      resourceConfig[key] = {
        translation: value,
      };
    });

    return {
      resources: resourceConfig,
      fallbackLng: "en",
      debug: true,
      detection: {
        order: [
          "navigator",
          "localStorage",
          "sessionStorage",
          "htmlTag",
          "path",
          "subdomain",
        ],
        caches: ["localStorage", "sessionStorage"],
        lookupLocalStorage: "i18nextLng",
        lookupSessionStorage: "i18nextLng",
      },
      interpolation: {
        escapeValue: false,
      },
    } as InitOptions;
  }
}
