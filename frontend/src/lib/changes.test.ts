import { describe, expect, it } from "vitest";

import type { ChangeView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { asksForALook, changeText } from "./changes";

const t = (key: string, vars?: Record<string, unknown>) => `${key} ${JSON.stringify(vars ?? {})}`;
const change = (c: Partial<ChangeView> & Pick<ChangeView, "kind" | "subject">): ChangeView => ({
  name: "", from: "", to: "", byVigie: false, super: false, finding: null, ...c,
});

describe("changes", () => {
  it("says each change in a sentence", () => {
    expect(changeText(change({ kind: "admin-added", subject: "user:root2", name: "root2", super: true }), t, "en")).toBe('changes.superAdded {"name":"root2"}');
    expect(changeText(change({ kind: "installed", subject: "plugin:wp-file-manager", name: "WP File Manager", to: "7.2.1" }), t, "en")).toBe(
      'changes.installed.plugin {"name":"WP File Manager","version":"7.2.1"}',
    );
    expect(changeText(change({ kind: "version", subject: "core", from: "7.1.3", to: "7.1.4" }), t, "en")).toBe(
      'changes.outside {"name":"update.group.core {}","from":"7.1.3","to":"7.1.4"}',
    );
    expect(changeText(change({ kind: "version", subject: "plugin:akismet", name: "Akismet", from: "5.3", to: "5.7.2", byVigie: true }), t, "en")).toContain("changes.byVigie");
    expect(changeText(change({ kind: "theme-switched", subject: "theme:astra", from: "Twenty Twenty-Four", to: "Astra" }), t, "en")).toBe(
      'changes.themeSwitched {"from":"Twenty Twenty-Four","to":"Astra"}',
    );
    const files = { key: "", subject: "core", name: "", identifier: "", version: "", ignore: null };
    expect(changeText(change({ kind: "files", subject: "core", finding: { ...files, kind: "core-file", level: "attention", facts: { changed: "2", added: "1", missing: "0" } } }), t, "en")).toBe(
      'changes.files {"name":"update.group.core {}","counts":"changes.counts.changed {\\"count\\":2}, changes.counts.added {\\"count\\":1}"}',
    );
  });

  it("asks for a look for what a person in charge would check first", () => {
    expect(["admin-added", "installed", "files", "finding", "admin-removed", "removed", "theme-switched", "version"].map((kind) => asksForALook(change({ kind, subject: "" })))).toEqual([
      true, true, true, true, false, false, false, false,
    ]);
  });
});
