// What changed on a site since its previous complete scan (ADR-0027), in words.
import type { ChangeView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "./findings";

type T = (key: string, vars?: Record<string, unknown>) => string;

/** The changes that ask for a look; the others are said in gray. */
const looks = new Set(["admin-added", "installed", "files", "finding"]);

/** Whether a change asks for a look: a new administrator, something installed, files, a new finding. */
export function asksForALook(c: ChangeView): boolean {
  return looks.has(c.kind);
}

/** A change in one sentence. */
export function changeText(c: ChangeView, t: T, lang: string): string {
  const kind = c.subject.split(":")[0];
  const name = c.subject === "core" ? t("update.group.core") : c.name;
  switch (c.kind) {
    case "admin-added":
      return t(c.super ? "changes.superAdded" : "changes.adminAdded", { name });
    case "admin-removed":
      return t("changes.adminRemoved", { name });
    case "installed":
    case "removed":
      return t(`changes.${c.kind}.${kind}`, { name, version: c.kind === "installed" ? c.to : c.from });
    case "theme-switched":
      return t("changes.themeSwitched", { from: c.from, to: c.to });
    case "version":
      return t(c.byVigie ? "changes.byVigie" : "changes.outside", { name, from: c.from, to: c.to });
    case "files":
      return filesText(c, name, t, lang);
    case "finding":
      if (!c.finding) return "";
      if (c.finding.kind === "vulnerability") {
        return t("changes.vulnerability", { name: c.finding.name, version: c.finding.version, title: c.finding.facts?.name ?? "" });
      }
      return t("changes.finding", { title: findingText(c.finding, t, lang).title });
  }
  return "";
}

/** Files newly different: how many changed, went missing or were added; a file of its own (one
 often deleted on purpose) by its finding. */
function filesText(c: ChangeView, name: string, t: T, lang: string): string {
  const f = c.finding;
  if (!f) return "";
  if (f.identifier) return t("changes.finding", { title: findingText(f, t, lang).title });
  const counts = (["changed", "missing", "added"] as const)
    .map((k) => [k, Number(f.facts?.[k] ?? 0)] as const)
    .filter(([, n]) => n > 0)
    .map(([k, n]) => t(`changes.counts.${k}`, { count: n }));
  return t("changes.files", { name: f.kind === "core-file" ? t("update.group.core") : name, counts: counts.join(", ") });
}
