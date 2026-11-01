import { describe, expect, it } from "vitest";

import { tagChoices } from "./tags";
import { fold } from "./text";

const used = [
  { name: "Contrat de maintenance", sites: 6 },
  { name: "Prioritaire", sites: 2 },
  { name: "Priorité basse", sites: 1 },
  { name: "Client Dupont", sites: 3 },
];

describe("tags", () => {
  it("folds case and accents", () => {
    expect(fold("Priorité ÉCOLES")).toBe("priorite ecoles");
  });

  it("offers the tags in use that hold what is typed, then a new one", () => {
    expect(tagChoices(used, [], "pri")).toEqual([
      { name: "Prioritaire", sites: 2, isNew: false },
      { name: "Priorité basse", sites: 1, isNew: false },
      { name: "pri", sites: 0, isNew: true },
    ]);
    // Starting with it comes first; the site's own tags are not offered.
    expect(tagChoices(used, ["prioritaire"], "ri").map((c) => c.name)).toEqual(["Priorité basse", "ri"]);
  });

  it("offers no new tag when one in use is what was typed", () => {
    expect(tagChoices(used, [], " PRIORITAIRE ")).toEqual([{ name: "Prioritaire", sites: 2, isNew: false }]);
    expect(tagChoices(used, ["Client Dupont"], "client dupont")).toEqual([]);
    expect(tagChoices(used, [], "  ")).toEqual([]);
  });
});
