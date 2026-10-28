// Translation catalogs shared with the Go side (../locales); English is the fallback.
import i18next, { type i18n } from "i18next";
import { initReactI18next } from "react-i18next";

import en from "../../locales/en.json";
import fr from "../../locales/fr.json";

export const languages = ["en", "fr"] as const;

/** A ready i18next instance for lang; unsupported languages fall back to English. */
export function createI18n(lang: string): i18n {
  const instance = i18next.createInstance();
  void instance.use(initReactI18next).init({
    lng: lang,
    fallbackLng: "en",
    supportedLngs: [...languages],
    resources: { en: { translation: en }, fr: { translation: fr } },
    interpolation: { escapeValue: false }, // React escapes text itself
    initAsync: false,
  });
  return instance;
}
