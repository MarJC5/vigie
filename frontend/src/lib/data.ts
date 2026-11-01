// What the Data tab shows of an export or an import: sizes, counts, the lines of a preview.
import type { TFunction } from "i18next";

import type { ImportItem, Node } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import type { IconName } from "../components/Icon/Icon";

/** A file size, in kilobytes under a megabyte: "340 kB", "3.1 MB". */
export function sizeText(bytes: number, lang: string): string {
  const mega = bytes >= 1_000_000;
  return new Intl.NumberFormat(lang, {
    style: "unit",
    unit: mega ? "megabyte" : "kilobyte",
    unitDisplay: "short",
    maximumFractionDigits: mega ? 1 : 0,
  }).format(Math.max(1, mega ? bytes / 1_000_000 : bytes / 1_000));
}

/** Counts joined in a sentence: "3 folders, 6 hosts, 11 sites"; the ones at zero are left out
 unless all are. */
export function countsText(t: TFunction, counts: Array<[string, number]>): string {
  const shown = counts.filter(([, n]) => n > 0);
  return (shown.length ? shown : counts.slice(0, 1)).map(([key, count]) => t(`data.count.${key}`, { count })).join(", ");
}

/** The folders an import can add into, by path, after the top. */
export function folderOptions(t: TFunction, nodes: Node[]): Array<{ value: string; label: string }> {
  const folders = nodes.filter((n) => n.kind === "folder").map((n) => ({ value: n.id, label: n.path }));
  return [{ value: "", label: t("data.top") }, ...folders.sort((a, b) => a.label.localeCompare(b.label))];
}

/** The folders and hosts an export can hold alone, by path. */
export function exportOptions(nodes: Node[]): Array<{ value: string; label: string }> {
  return nodes.filter((n) => n.kind !== "site").map((n) => ({ value: n.id, label: n.path }));
}

export type ImportGroup = "added" | "present" | "refused";

/** The items of a preview by what the import does with them, in their order. */
export function groupsOf(items: ImportItem[]): Record<ImportGroup, ImportItem[]> {
  const groups: Record<ImportGroup, ImportItem[]> = { added: [], present: [], refused: [] };
  for (const item of items) groups[item.status as ImportGroup]?.push(item);
  return groups;
}

export function itemIcon(item: ImportItem): IconName {
  switch (item.kind) {
    case "folder":
      return "folder";
    case "host":
      return "host";
    case "site":
      return "sites";
  }
  return "warning";
}

/** The name of an item: its path, or for a line of a CSV list, its number and why it is refused. */
export function itemName(t: TFunction, item: ImportItem): string {
  if (item.kind !== "line") return item.path;
  const reason = t(`cli.import.line.${item.reason}`);
  return t("data.preview.line", { line: item.line, reason: item.value ? `${reason} (${item.value})` : reason });
}

/** The line under an item's name: what it is, or why it is there or refused. */
export function itemDetail(t: TFunction, item: ImportItem): string {
  if (item.kind === "line") return item.detail;
  if (item.status === "refused") return item.reason === "invalid" ? item.detail : t(`data.reason.${item.reason}`);
  if (item.status === "present") return t(`data.reason.${item.reason}`);
  const parts: string[] = [];
  switch (item.kind) {
    case "folder":
      parts.push(t("data.preview.newFolder"));
      break;
    case "host":
      parts.push(t("data.preview.host"), item.account);
      break;
    case "site":
      parts.push(t("data.preview.site"), item.sitePath);
      if (item.subsites) parts.push(t("data.preview.subsites", { count: item.subsites }));
      if (item.scans || item.operations) {
        parts.push(t("data.preview.history", { counts: countsText(t, [["scans", item.scans], ["operations", item.operations]]) }));
      }
      break;
  }
  if (item.reason === "renamed") parts.push(t("data.reason.renamed"));
  for (const l of item.tagsLeftOut ?? []) parts.push(t("data.preview.tagLeftOut", { tag: l.tag, reason: t(`cli.import.tag.${l.reason}`) }));
  return parts.join(" · ");
}
