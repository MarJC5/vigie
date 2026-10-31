// The docs of the repository (docs/*.md, translated in docs/fr/), bundled at build time: the help
// works offline (ADR-0020). docs/docs_test.go keeps both languages in step.
import addons from "../../../../docs/addons.md?raw";
import background from "../../../../docs/background.md?raw";
import data from "../../../../docs/data.md?raw";
import addonsFr from "../../../../docs/fr/addons.md?raw";
import backgroundFr from "../../../../docs/fr/background.md?raw";
import dataFr from "../../../../docs/fr/data.md?raw";
import securityFr from "../../../../docs/fr/security.md?raw";
import updatesFr from "../../../../docs/fr/updates.md?raw";
import security from "../../../../docs/security.md?raw";
import updates from "../../../../docs/updates.md?raw";

/** A doc of the help, by the name of its file. */
export type Doc = "security" | "updates" | "data" | "addons" | "background";

export const docNames: Doc[] = ["security", "updates", "data", "addons", "background"];

const texts: Record<string, Record<Doc, string>> = {
  en: { security, updates, data, addons, background },
  fr: { security: securityFr, updates: updatesFr, data: dataFr, addons: addonsFr, background: backgroundFr },
};

/** A doc in the language asked, else in English. */
export function docText(doc: Doc, lang: string): string {
  return (texts[lang] ?? texts.en!)[doc];
}

/** Where the docs are published, for a link to a file of the repository. */
const repository = "https://github.com/MarJC5/vigie/blob/main/";

/**
 * What a link of a doc leads to: another doc (its tab), or an address for the browser; a file of
 * the repository opens on GitHub. Undefined for an address the window does not open.
 */
export function linkTarget(href: string, lang: string): { doc: Doc } | { url: string } | undefined {
  if (/^https?:\/\//.test(href)) return { url: href };
  if (/^[a-z]+:/i.test(href)) return undefined;
  const name = href.match(/^([a-z]+)\.md(?:#.*)?$/)?.[1];
  if (name && (docNames as string[]).includes(name)) return { doc: name as Doc };
  return { url: new URL(href, repository + (lang === "fr" ? "docs/fr/" : "docs/")).href };
}
