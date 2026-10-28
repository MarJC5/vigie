// The interface picks plural forms with Intl.PluralRules (through i18next); these cases are the
// same ones the Go side checks, so both sides agree on every count.
import { describe, expect, it } from "vitest";

import en from "../../locales/en.json";
import fr from "../../locales/fr.json";
import cases from "../../locales/testdata/plurals.json";
import { createI18n } from "../src/i18n";

const catalogs = { en, fr } as const;

describe.each(["en", "fr"] as const)("%s", (lang) => {
  const i18n = createI18n(lang);
  const counts = Object.entries(cases[lang]) as Array<[string, string]>;

  it.each(counts)("uses the expected plural form for %s", (count, category) => {
    const running = catalogs[lang].manager as Record<string, string>;
    const expected = running[`running_${category}`]!.replace("{{count}}", count);
    expect(i18n.t("manager.running", { count: Number(count) })).toBe(expected);
  });
});

describe("createI18n", () => {
  it("falls back to English for an unsupported language", () => {
    expect(createI18n("de").t("nav.activity")).toBe("Activity");
  });

  it("translates into French", () => {
    expect(createI18n("fr").t("nav.activity")).toBe("Activité");
  });
});
