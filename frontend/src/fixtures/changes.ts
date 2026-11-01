// Fictional changes since a site's previous scan (ADR-0027), as the Go side serves them.
import type { ChangeView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { exploited } from "./findings";

const change = (c: Partial<ChangeView> & Pick<ChangeView, "kind" | "subject">): ChangeView => ({
  name: "", from: "", to: "", byVigie: false, super: false, finding: null, ...c,
});

/** What a scan of boulangerie-exemple.ch found changed: what asks for a look first. */
export const siteChanges: ChangeView[] = [
  change({ kind: "admin-added", subject: "user:marie.dupont", name: "marie.dupont" }),
  change({ kind: "admin-removed", subject: "user:stagiaire", name: "stagiaire" }),
  change({ kind: "installed", subject: "plugin:wp-file-manager", name: "WP File Manager", to: "7.2.1" }),
  change({ kind: "theme-switched", subject: "theme:astra", name: "Astra", from: "Twenty Twenty-Four", to: "Astra" }),
  change({ kind: "version", subject: "core", from: "7.1.3", to: "7.1.4" }),
  change({ kind: "version", subject: "plugin:akismet", name: "Akismet Anti-spam", from: "5.3", to: "5.7.2", byVigie: true }),
  change({
    kind: "files", subject: "core",
    finding: { key: "core-file|core|", kind: "core-file", level: "attention", subject: "core", name: "", identifier: "", version: "", facts: { changed: "2", added: "1", missing: "0" }, ignore: null },
  }),
  change({ kind: "finding", subject: exploited.subject, name: exploited.name, finding: exploited }),
];
