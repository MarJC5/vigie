// The short topics of the help (docs/help/*.md, translated in docs/fr/help/), bundled at build time:
// the help works offline (ADR-0020, ADR-0021). docs/docs_test.go keeps both languages in step and
// each topic short; the full docs stay on GitHub, where each topic links.
import background from "../../../../docs/help/background.md?raw";
import copies from "../../../../docs/help/copies.md?raw";
import importExport from "../../../../docs/help/import-export.md?raw";
import server from "../../../../docs/help/server.md?raw";
import status from "../../../../docs/help/status.md?raw";
import update from "../../../../docs/help/update.md?raw";
import backgroundFr from "../../../../docs/fr/help/background.md?raw";
import copiesFr from "../../../../docs/fr/help/copies.md?raw";
import importExportFr from "../../../../docs/fr/help/import-export.md?raw";
import serverFr from "../../../../docs/fr/help/server.md?raw";
import statusFr from "../../../../docs/fr/help/status.md?raw";
import updateFr from "../../../../docs/fr/help/update.md?raw";

/** A topic of the help, by the name of its file. */
export type Topic = "update" | "status" | "server" | "import-export" | "copies" | "background";

export const topicNames: Topic[] = ["update", "status", "server", "import-export", "copies", "background"];

const texts: Record<string, Record<Topic, string>> = {
  en: { update, status, server, "import-export": importExport, copies, background },
  fr: { update: updateFr, status: statusFr, server: serverFr, "import-export": importExportFr, copies: copiesFr, background: backgroundFr },
};

/** A topic in the language asked, else in English. */
export function topicText(topic: Topic, lang: string): string {
  return (texts[lang] ?? texts.en!)[topic];
}

/** Where the docs are published, for a link to a full doc or a file of the repository. */
const repository = "https://github.com/MarJC5/vigie/blob/main/";

/**
 * What a link of a topic leads to: another topic (its tab), or an address for the browser; a doc
 * of the repository opens on GitHub. Undefined for an address the window does not open.
 */
export function linkTarget(href: string, lang: string): { topic: Topic } | { url: string } | undefined {
  if (/^https?:\/\//.test(href)) return { url: href };
  if (/^[a-z]+:/i.test(href)) return undefined;
  const name = href.match(/^([a-z-]+)\.md$/)?.[1];
  if (name && (topicNames as string[]).includes(name)) return { topic: name as Topic };
  return { url: new URL(href, repository + (lang === "fr" ? "docs/fr/help/" : "docs/help/")).href };
}
