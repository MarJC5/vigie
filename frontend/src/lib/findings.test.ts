import { describe, expect, it } from "vitest";

import en from "../../../locales/en.json";
import fr from "../../../locales/fr.json";
import { certificate, closed, domainSilent, exploited, findings, hidden, notChecked, noTwoFactor, sameName, xss } from "../fixtures/findings";
import { createI18n } from "../i18n";
import { day, findingText, groupsOf, ignoreText, keyOf } from "./findings";

const t = (lang: string) => createI18n(lang).t as (key: string, vars?: Record<string, unknown>) => string;

describe("findingText", () => {
  it("writes a vulnerability with its score, exploitation, fix and record", () => {
    expect(findingText(exploited, t("fr"), "fr")).toEqual({
      title: "WooCommerce 9.8.1\u00a0: SQL injection without authentication",
      detail: "CVSS 9,8, critique · exploitée (KEV) · corrigée en 9.8.3",
      link: { label: "CVE-2026-41234", href: "https://www.cve.org/CVERecord?id=CVE-2026-41234" },
    });
    expect(findingText(xss, t("en"), "en").detail).toBe("CVSS 5.4, medium · fixed in 6.0.6");
  });

  it("says when a vulnerability has no score, or no fix", () => {
    const unscored = { ...exploited, facts: { name: "x", unfixed: "true" } };
    expect(findingText(unscored, t("en"), "en").detail).toBe("severity unknown · not fixed in any version");
  });

  it("writes the other kinds from their facts", () => {
    expect(findingText(closed, t("fr"), "fr")).toMatchObject({
      title: "Old Gallery est fermée sur wordpress.org",
      detail: "Fermée le 30.01.2026\u00a0: demande de l'auteur",
      link: { href: "https://wordpress.org/plugins/old-gallery/" },
    });
    expect(findingText(noTwoFactor, t("en"), "en")).toEqual({
      title: "marie: administrator without two-factor",
      detail: "marie@boulangerie-exemple.ch · since 12.03.2024",
    });
    expect(findingText(sameName, t("en"), "en")).toEqual({
      title: "Gutenblocks: 232 files differ from wordpress.org's",
      detail:
        "231 added, 1 changed · including app/blocks/home-hero/block.json, gutenblocks.php · Nearly every file differs: probably another plugin than the one of the same name on wordpress.org.",
    });
    expect(findingText(domainSilent, t("fr"), "fr").detail).toBe("Expiration non publiée par le registre du .ch");
    expect(findingText(certificate, t("en"), "en").title).toBe("TLS certificate valid until 25.12.2026");
    expect(findingText(notChecked, t("fr"), "fr").title).toBe("wordpress.org non interrogé");
  });

  it("has a title in both languages for every finding", () => {
    for (const lang of ["en", "fr"]) {
      for (const f of findings) {
        const text = findingText(f, t(lang), lang);
        expect(text.title, f.key).not.toMatch(/security\.finding|undefined/);
        expect(text.detail, f.key).not.toMatch(/security\.finding|undefined/);
      }
    }
  });
});

describe("groupsOf", () => {
  it("puts the findings by level, the ignored ones last", () => {
    expect(groupsOf(findings).map((g) => [g.group, g.items.length])).toEqual([
      ["urgent", 2],
      ["attention", 5],
      ["not-checked", 1],
      ["info", 2],
      ["ignored", 1],
    ]);
  });
});

describe("ignoreText", () => {
  it("says since when and why", () => {
    expect(ignoreText(hidden, t("fr"))).toBe("Ignoré définitivement le 12.10.2026\u00a0: «\u00a0staging copy, hidden on purpose\u00a0»");
    const untilNext = { ...xss, ignore: { untilVersion: "6.0.4", reason: "", created: "2026-10-30T14:12:00Z" } };
    expect(ignoreText(untilNext, t("en"))).toBe("Ignored on 30.10.2026 until a version other than 6.0.4");
  });
});

describe("the catalogs", () => {
  it("have the same security texts in both languages", () => {
    const keys = (o: object, prefix = ""): string[] =>
      Object.entries(o).flatMap(([k, v]) => (typeof v === "object" ? keys(v as object, `${prefix}${k}.`) : [`${prefix}${k}`]));
    // French has a plural form more ("_many"): compare without it.
    const plain = (list: string[]) => list.filter((k) => !k.endsWith("_many")).sort();
    for (const section of ["security", "lookups", "consent", "settings"] as const) {
      expect(plain(keys(fr[section], `${section}.`))).toEqual(plain(keys(en[section], `${section}.`)));
    }
  });

  it("name every check and destination without a dot", () => {
    expect(keyOf("wordpress.org")).toBe("wordpressOrg");
    expect(day("2026-10-30T14:12:00Z")).toBe("30.10.2026");
  });
});
