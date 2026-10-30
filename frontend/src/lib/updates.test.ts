import { describe, expect, it } from "vitest";

import { outdatedSite } from "../fixtures/scans";
import { bulk, completed, failed, preview, rolledBack, running } from "../fixtures/updates";
import {
  backupPath,
  bulkCounts,
  choicesOf,
  durationText,
  failedStep,
  itemsDone,
  itemsOf,
  namesOf,
  offeredKeys,
  plannedItems,
  rescanRow,
  restoredStep,
  rowText,
  runRows,
  seconds,
  selectionOf,
  selectionOfItems,
} from "./updates";

describe("choicesOf", () => {
  const groups = choicesOf(outdatedSite.scan!.updates!);
  it("groups the updates, the translations in one row", () => {
    expect(groups.map((g) => [g.kind, g.choices.map((c) => c.key)])).toEqual([
      ["core", ["core"]],
      ["plugin", ["plugin:akismet", "plugin:woocommerce", "plugin:old-gallery"]],
      ["theme", []],
      ["language", ["languages"]],
    ]);
    expect(groups[3]!.choices[0]).toMatchObject({ languages: ["fr_FR"], of: ["", "akismet"] });
  });
  it("names a plugin by its title and keeps why one is not offered", () => {
    expect(groups[1]!.choices[0]).toMatchObject({ name: "Akismet Anti-spam", from: "5.3", to: "5.7.2", offered: true });
    expect(groups[1]!.choices[2]).toMatchObject({ offered: false, reason: "unavailable" });
  });
  it("chooses every item offered", () => {
    expect([...offeredKeys(groups)]).toEqual(["core", "plugin:akismet", "plugin:woocommerce", "languages"]);
  });
});

describe("selectionOf", () => {
  it("turns keys into what Vigie updates", () => {
    expect(selectionOf(["plugin:akismet", "languages", "theme:twentyfive", "core"])).toEqual({
      core: true,
      plugins: ["akismet"],
      themes: ["twentyfive"],
      languages: true,
    });
  });
});

describe("selectionOfItems", () => {
  it("sends only what the preview still offers", () => {
    // Twenty Twenty-Five was chosen from the scan, but the preview no longer offers it.
    const items = plannedItems(preview, selectionOf(["core", "plugin:akismet", "theme:twentytwentyfive", "languages"]));
    expect(selectionOfItems(items)).toEqual({ core: true, plugins: ["akismet"], themes: [], languages: true });
  });
});

describe("plannedItems", () => {
  it("keeps the items chosen, in the order of the run", () => {
    const items = plannedItems(preview, selectionOf(["languages", "plugin:woocommerce"]));
    expect(items.map((it) => `${it.kind}:${it.slug}`)).toEqual(["plugin:woocommerce", "language:", "language:akismet"]);
  });
});

describe("runRows", () => {
  const planned = plannedItems(preview, selectionOf(["plugin:akismet", "plugin:woocommerce", "languages"]));
  it("shows the planned items before the run has a step", () => {
    expect(runRows(null, planned).map((r) => [r.key, r.state])).toEqual([
      ["item-plugin:akismet", "waiting"],
      ["item-plugin:woocommerce", "waiting"],
      ["item-languages", "waiting"],
    ]);
  });
  it("follows the steps, then what is left", () => {
    const rows = runRows(running, planned);
    expect(rows.map((r) => r.state)).toEqual(["done", "done", "done", "done", "done", "running", "waiting"]);
    expect(rows.at(-1)!.item).toMatchObject({ kind: "language" });
  });
  it("counts the items done, the translations once their last step is over", () => {
    expect(itemsDone(null)).toBe(0);
    expect(itemsDone(running)).toBe(1);
    expect(itemsDone(failed)).toBe(1);
    expect(itemsDone(completed)).toBe(1);
    const theme = { ...completed.steps![4]!, position: 5, slug: "theme" };
    expect(itemsDone({ ...completed, steps: [...completed.steps!, theme] })).toBe(2);
  });
  it("ends a run with the scan of the site, under way once every step is over", () => {
    expect(rescanRow(runRows(running, planned))).toMatchObject({ state: "waiting", rescan: true });
    expect(rescanRow(runRows({ ...running, steps: running.steps!.slice(0, 5) }, []))).toMatchObject({ state: "running" });
  });
  it("marks the step that failed", () => {
    expect(runRows(failed, planned).map((r) => r.state).slice(-2)).toEqual(["failed", "waiting"]);
    expect(failedStep(failed)).toMatchObject({ step: "check", slug: "woocommerce" });
  });
});

describe("history", () => {
  it("counts the items of an update, the translations as one", () => {
    expect(itemsOf(failed)).toBe(2);
    expect(itemsOf(completed)).toBe(2);
  });
  it("finds what a rollback put back", () => {
    expect(restoredStep(rolledBack)).toMatchObject({ slug: "woocommerce", versionAfter: "9.8.1" });
  });
  it("shows a backups folder from the home", () => {
    expect(backupPath(".vigie/backups/a/20261029-141203")).toBe("~/.vigie/backups/a/20261029-141203");
    expect(backupPath("~/.vigie/backups/a/")).toBe("~/.vigie/backups/a/");
  });
  it("measures a step", () => {
    expect(seconds("2026-10-29T14:12:03Z", "2026-10-29T14:12:09Z")).toBe(6);
    expect(seconds("2026-10-29T14:12:03Z", "")).toBe(0);
  });
});

describe("bulkCounts", () => {
  it("counts the sites by outcome", () => {
    expect(bulkCounts(bulk)).toMatchObject({ updated: 1, failed: 1, "not-started": 2, refused: 1 });
  });
});

describe("rowText", () => {
  const t = (key: string, vars?: Record<string, unknown>) => (vars ? `${key} ${JSON.stringify(vars)}` : key);
  const names = namesOf(outdatedSite.scan!.updates!);
  const planned = plannedItems(preview, selectionOf(["plugin:akismet", "plugin:woocommerce", "languages"]));
  const rows = runRows(failed, planned);
  it("names an item by its title", () => {
    expect(rowText(t, rows[1]!, names, true).label).toBe('update.step.backup-files {"item":"Akismet Anti-spam"}');
    expect(rowText(t, rows[3]!, names, true).label).toBe('update.step.update {"item":"Akismet Anti-spam","from":"5.3","to":"5.7.2"}');
  });
  it("says why a check failed, and nothing when it passed", () => {
    expect(rowText(t, rows[4]!, names, true).detail).toBe("");
    expect(rowText(t, rows[6]!, names, true).detail).toContain("wc_get_page_id");
  });
  it("marks what a run that ended did not start", () => {
    expect(rowText(t, rows[7]!, names, true)).toEqual({ label: "update.step.languages", detail: "update.step.notStarted" });
    expect(rowText(t, rows[7]!, names, false).detail).toBe("");
  });
  it("writes durations", () => {
    expect(durationText(t, 6)).toBe('update.duration.seconds {"s":6}');
    expect(durationText(t, 72)).toBe('update.duration.minutes {"m":1,"s":12}');
  });
});
