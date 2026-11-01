import { describe, expect, it } from "vitest";

import { createI18n } from "../i18n";
import { csvPreview, vigiePreview } from "../fixtures/data";
import { sampleTree } from "../fixtures/tree";
import { countsText, exportOptions, folderOptions, groupsOf, itemDetail, itemName, sizeText } from "./data";

const t = createI18n("en").t;

describe("data", () => {
  it("writes sizes in kilobytes under a megabyte", () => {
    expect(sizeText(3_150_000, "en")).toBe("3.2 MB");
    expect(sizeText(412_000, "en")).toBe("412 kB");
    expect(sizeText(20, "en")).toBe("1 kB");
  });

  it("joins the counts that are not zero", () => {
    expect(countsText(t, [["folders", 1], ["hosts", 0], ["sites", 11]])).toBe("1 folder, 11 sites");
    expect(countsText(t, [["folders", 0], ["hosts", 0]])).toBe("0 folders");
  });

  it("offers the folders to import into, and the folders and hosts to export", () => {
    expect(folderOptions(t, sampleTree).map((o) => o.label)).toEqual(["Top level", "Clients", "Personal"]);
    expect(exportOptions(sampleTree).map((o) => o.value)).toEqual(["clients", "shared", "personal", "vps"]);
  });

  it("describes each item of a preview", () => {
    const groups = groupsOf(vigiePreview.items!);
    expect([groups.added.length, groups.present.length, groups.refused.length]).toEqual([5, 2, 0]);
    expect(groups.added.map((i) => itemDetail(t, i))).toEqual([
      "new folder",
      "host · atelier@ssh.example-host.ch",
      "site · ~/www/atelier · with 12 scans, 3 updates and rollbacks",
      'site · ~/www/ecoles · network, 3 subsites · tag "Écoles, primaires" left out (a comma, a vertical bar or a control character)',
      "host · deploy@203.0.113.24:2222 · its name was taken: renamed",
    ]);
    expect(itemDetail(t, groups.present[0]!)).toBe("same account");
    const line = groupsOf(csvPreview.items!).refused[1]!;
    expect(itemName(t, line)).toBe("Line 11: a port that is not one (22a)");
    expect(itemDetail(t, line)).toBe("Valais;ssh.example-host.ch;22a;atelier;~/www/test;");
  });
});
