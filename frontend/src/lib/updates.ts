// What the update screens show, worked out from the scan, the preview and the steps recorded.
import type {
  IgnoreView,
  OperationView,
  SelectionView,
  SitePreviewView,
  SiteResultView,
  StepView,
  UpdateItemView,
  UpdateView,
} from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { day } from "./findings";

export type ItemKind = "core" | "plugin" | "theme" | "language";

/** The key of an item to choose: "core", "plugin:<slug>", "theme:<slug>", or "languages" for all
 translations, which update together. */
export function keyOf(kind: string, slug: string): string {
  if (kind === "core") return "core";
  if (kind === "language") return "languages";
  return `${kind}:${slug}`;
}

/** Where the changes of an update are read (ADR-0026): a plugin wordpress.org knows, on the
 Development tab of its page; WordPress, in the release notes of the version offered (x.y.z, or
 x.y for a major one). Themes and other plugins have none there. */
export function changelogOf(item: { kind: string; slug: string; to: string; onWordPressOrg?: boolean }): string | undefined {
  if (item.kind === "plugin" && item.onWordPressOrg) return `https://wordpress.org/plugins/${item.slug}/#developers`;
  if (item.kind !== "core") return undefined;
  const version = /^(\d+)\.(\d+)(?:\.(\d+))?$/.exec(item.to);
  if (!version) return undefined;
  const [, major, minor, patch] = version;
  const parts = patch && patch !== "0" ? [major, minor, patch] : [major, minor];
  return `https://wordpress.org/documentation/wordpress-version/version-${parts.join("-")}/`;
}

/** One item of the Updates tab; an item not offered (newer PHP needed, state unknown) cannot be
 chosen. */
export interface Choice {
  key: string;
  kind: ItemKind;
  /** The plugin or theme's title, else its slug; empty for WordPress and translations. */
  name: string;
  from: string;
  to: string;
  offered: boolean;
  /** Why it is not offered: "unavailable" or "unknown". */
  reason: string;
  /** Translations only: the languages, and what they translate ("" for WordPress). */
  languages: string[];
  of: string[];
  /** Ignores the update with Security.Ignore (ADR-0026); "" when it cannot be ignored. */
  ignoreKey: string;
  /** Why and until when it is ignored; null when it counts. */
  ignore: IgnoreView | null;
  /** Where its changes are read, when wordpress.org has them. */
  changelog?: string;
}

/** The ignored updates go in a group of their own, after the others, unticked. */
export interface ChoiceGroup {
  kind: ItemKind | "ignored";
  choices: Choice[];
}

const kinds: ItemKind[] = ["core", "plugin", "theme", "language"];

/** The updates of a scan as the tab lists them: WordPress, plugins, themes, then all translations in
 one row; the ignored ones last, in a group of their own. */
export function choicesOf(updates: UpdateView[]): ChoiceGroup[] {
  const choice = (u: UpdateView): Choice => ({
    key: keyOf(u.kind, u.slug),
    kind: u.kind as ItemKind,
    name: u.title || u.slug,
    from: u.version,
    to: u.updateVersion,
    offered: u.update === "available",
    reason: u.update === "available" ? "" : u.update,
    languages: [],
    of: [],
    ignoreKey: u.key,
    ignore: u.ignore,
    changelog: u.update === "available" ? changelogOf({ kind: u.kind, slug: u.slug, to: u.updateVersion, onWordPressOrg: u.onWordPressOrg }) : undefined,
  });
  const ignored = updates.filter((u) => u.ignore);
  const groups: ChoiceGroup[] = kinds.map((kind) => {
    const of = updates.filter((u) => u.kind === kind && !u.ignore);
    if (kind !== "language") return { kind, choices: of.map(choice) };
    const offered = of.filter((u) => u.update === "available");
    if (offered.length === 0) return { kind, choices: [] };
    return {
      kind,
      choices: [
        {
          key: "languages",
          kind,
          name: "",
          from: "",
          to: "",
          offered: true,
          reason: "",
          languages: unique(offered.map((u) => u.language)),
          of: unique(offered.map((u) => u.slug)),
          ignoreKey: "",
          ignore: null,
        },
      ],
    };
  });
  return ignored.length > 0 ? [...groups, { kind: "ignored", choices: ignored.map(choice) }] : groups;
}

/** The keys of every item offered, chosen when the tab opens; an ignored update only on purpose. */
export function offeredKeys(groups: ChoiceGroup[]): Set<string> {
  return new Set(groups.filter((g) => g.kind !== "ignored").flatMap((g) => g.choices.filter((c) => c.offered).map((c) => c.key)));
}

/** How long an update is ignored, and why. */
export function ignoredText(ignore: IgnoreView, t: (key: string, vars?: Record<string, unknown>) => string): string {
  const date = day(ignore.created);
  return ignore.untilVersion ? t("update.ignoredUntil", { version: ignore.untilVersion, date }) : t("update.ignoredForever", { date, reason: ignore.reason });
}

/** The selection sent to Vigie for the keys chosen. */
export function selectionOf(keys: Iterable<string>): SelectionView {
  const sel: SelectionView = { core: false, plugins: [], themes: [], languages: false };
  for (const key of keys) {
    if (key === "core") sel.core = true;
    else if (key === "languages") sel.languages = true;
    else if (key.startsWith("plugin:")) sel.plugins!.push(key.slice("plugin:".length));
    else if (key.startsWith("theme:")) sel.themes!.push(key.slice("theme:".length));
  }
  return sel;
}

/** The selection of exactly these items, so nothing the preview no longer offers is sent. */
export function selectionOfItems(items: UpdateItemView[]): SelectionView {
  return {
    core: items.some((it) => it.kind === "core"),
    plugins: items.filter((it) => it.kind === "plugin").map((it) => it.slug),
    themes: items.filter((it) => it.kind === "theme").map((it) => it.slug),
    languages: items.some((it) => it.kind === "language"),
  };
}

/** The items of a preview that the selection keeps, in the order Vigie updates them: WordPress, the
 plugins, the themes, then the translations. */
export function plannedItems(preview: SitePreviewView, sel: SelectionView): UpdateItemView[] {
  const items: UpdateItemView[] = [];
  if (sel.core && preview.core) items.push(preview.core);
  items.push(...(preview.plugins ?? []).filter((it) => sel.plugins?.includes(it.slug)));
  items.push(...(preview.themes ?? []).filter((it) => sel.themes?.includes(it.slug)));
  if (sel.languages) items.push(...(preview.languages ?? []));
  return items;
}

/** How many items "Update all" takes from a preview, the ignored ones aside; the translations
 count as one. */
export function pendingOf(p: SitePreviewView): number {
  const counted = (list: UpdateItemView[] | null | undefined) => (list ?? []).filter((it) => !it.ignored).length;
  return (p.core && !p.core.ignored ? 1 : 0) + counted(p.plugins) + counted(p.themes) + ((p.languages ?? []).length > 0 ? 1 : 0);
}

/** The updates of a preview the site ignores, which "Update all" leaves out. */
export function ignoredOf(p: SitePreviewView): UpdateItemView[] {
  return [...(p.core ? [p.core] : []), ...(p.plugins ?? []), ...(p.themes ?? [])].filter((it) => it.ignored);
}

/** Can an update of this preview start: it was read, the server can dump, something is chosen. */
export function canStart(preview: SitePreviewView, planned: UpdateItemView[]): boolean {
  return !preview.problem && preview.dumpTool && planned.length > 0;
}

export type RowState = "done" | "running" | "failed" | "waiting";

/** One row of a run: a step recorded, an item not reached yet, or the scan that ends a run. */
export interface RunRow {
  key: string;
  state: RowState;
  step?: StepView;
  item?: UpdateItemView;
  rescan?: boolean;
}

/** The last row of a run going on: once every step is over, Vigie removes the older backups and
 scans the site again, which the steps do not show. */
export function rescanRow(rows: RunRow[]): RunRow {
  const over = rows.length > 0 && rows.every((r) => r.state === "done");
  return { key: "rescan", state: over ? "running" : "waiting", rescan: true };
}

/** The rows of a run: its steps so far, then the planned items it has not reached. */
export function runRows(op: OperationView | null, planned: UpdateItemView[]): RunRow[] {
  const steps = op?.steps ?? [];
  const rows: RunRow[] = steps.map((step) => ({ key: `step-${step.position}`, state: stateOf(step), step }));
  const reached = (it: UpdateItemView) =>
    steps.some(
      (st) =>
        st.step === "update" &&
        st.itemKind === it.kind &&
        (it.kind === "language" || it.kind === "core" || st.slug === it.slug),
    );
  let languages = false;
  for (const it of planned) {
    if (it.kind === "language") {
      if (languages) continue;
      languages = true;
    }
    if (!reached(it)) rows.push({ key: `item-${keyOf(it.kind, it.slug)}`, state: "waiting", item: it });
  }
  return rows;
}

function stateOf(step: StepView): RowState {
  switch (step.outcome) {
    case "done":
      return "done";
    case "failed":
      return "failed";
    case "running":
      return "running";
  }
  return "waiting";
}

/** The items of a run done so far: each one checked after its update, and the translations once
 the last of their three steps (those of the themes) is over. */
export function itemsDone(op: OperationView | null): number {
  const steps = op?.steps ?? [];
  const checked = steps.filter((st) => st.step === "check" && st.outcome === "done").length;
  const translated = steps.some((st) => st.step === "update" && st.itemKind === "language" && st.slug === "theme" && st.outcome === "done");
  return checked + (translated ? 1 : 0);
}

/** The last step that failed in an operation. */
export function failedStep(op: OperationView): StepView | undefined {
  return (op.steps ?? []).filter((st) => st.outcome === "failed").at(-1);
}

/** The items an operation changed or set out to change; the translations count as one. */
export function itemsOf(op: OperationView): number {
  const keys = new Set(
    (op.steps ?? [])
      .filter((st) => (st.step === "backup-files" || st.step === "update") && st.itemKind !== "")
      .map((st) => keyOf(st.itemKind, st.slug)),
  );
  return keys.size;
}

/** The step of a rollback that put the item back, which names it and its version. */
export function restoredStep(op: OperationView): StepView | undefined {
  return (op.steps ?? []).find((st) => st.step === "restore-files");
}

/** A backups folder as the window shows it: Vigie stores it relative to the account's home. */
export function backupPath(dir: string): string {
  return dir.startsWith("/") || dir.startsWith("~") ? dir : `~/${dir}`;
}

/** Seconds between two times of Vigie (RFC 3339); 0 while the second is empty. */
export function seconds(started: string, finished: string): number {
  if (!started || !finished) return 0;
  return Math.max(0, Math.round((new Date(finished).getTime() - new Date(started).getTime()) / 1000));
}

export type BulkOutcome = "updated" | "failed" | "refused" | "nothing" | "not-started" | "cancelled";

/** How many sites of a bulk run ended each way. */
export function bulkCounts(results: SiteResultView[]): Record<BulkOutcome, number> {
  const counts: Record<BulkOutcome, number> = {
    updated: 0,
    failed: 0,
    refused: 0,
    nothing: 0,
    "not-started": 0,
    cancelled: 0,
  };
  for (const r of results) {
    if (r.outcome in counts) counts[r.outcome as BulkOutcome]++;
  }
  return counts;
}

type T = (key: string, vars?: Record<string, unknown>) => string;

/** The titles of the plugins and themes of a scan, by key, to name them better than by slug. */
export function namesOf(updates: UpdateView[]): Map<string, string> {
  return new Map(updates.filter((u) => u.title).map((u) => [keyOf(u.kind, u.slug), u.title]));
}

/** The keys of the plugins wordpress.org knows, whose changelog is there. */
export function knownOf(updates: UpdateView[]): Set<string> {
  return new Set(updates.filter((u) => u.onWordPressOrg).map((u) => keyOf(u.kind, u.slug)));
}

/** An item's name: WordPress, its title when known, else its slug. */
export function itemName(t: T, kind: string, slug: string, names: Map<string, string>): string {
  if (kind === "core") return t("update.group.core");
  return names.get(keyOf(kind, slug)) ?? slug;
}

/** The label and detail of a row of a run. */
export function rowText(t: T, row: RunRow, names: Map<string, string>, ended: boolean): { label: string; detail: string } {
  if (row.rescan) return { label: t("update.step.rescan"), detail: row.state === "running" ? t("update.step.rescanDetail") : "" };
  if (row.item) {
    const it = row.item;
    const label =
      it.kind === "language"
        ? t("update.step.languages")
        : t("update.step.update", { item: itemName(t, it.kind, it.slug, names), from: it.from, to: it.to });
    return { label, detail: ended ? t("update.step.notStarted") : "" };
  }
  const st = row.step!;
  const item = itemName(t, st.itemKind, st.slug, names);
  const failure = st.outcome === "failed" ? st.message : "";
  switch (st.step) {
    case "backup-files":
      return { label: st.itemKind === "core" ? t("update.step.backup-core") : t("update.step.backup-files", { item }), detail: failure };
    case "update":
      if (st.itemKind === "language") return { label: t(`update.step.languagesOf.${st.slug}`), detail: failure };
      return {
        label: t("update.step.update", { item, from: st.versionBefore, to: st.versionAfter }),
        detail: failure || (st.outcome === "done" ? st.message : ""),
      };
    case "check":
      // A check that passed says nothing more than its mark.
      return { label: t("update.step.check"), detail: failure };
    case "restore-files":
      return { label: t("update.step.restore-files", { item, version: st.versionAfter }), detail: failure };
  }
  return { label: t(`update.step.${st.step}`), detail: failure };
}

/** A duration as the screens show it: "6 s", "1 min 12 s". */
export function durationText(t: T, secs: number): string {
  if (secs < 60) return t("update.duration.seconds", { s: secs });
  return t("update.duration.minutes", { m: Math.floor(secs / 60), s: secs % 60 });
}

function unique(list: string[]): string[] {
  return [...new Set(list)];
}
