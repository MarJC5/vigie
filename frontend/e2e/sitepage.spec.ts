import { expect, test } from "@playwright/test";

// The site page complete (ADR-0023): its quick links, notes written and found by the search, a
// host's table sorted and filtered.
const seeded = "http://localhost:34115/";

test("a site's notes are written, then found by the search; a host's table sorts and filters", async ({ page, browserName }) => {
  await page.goto(seeded);
  // Both browsers run at once on the same data: each writes the notes of its own site.
  const [site, other] = browserName === "webkit" ? ["atelier-exemple.ch", "boulangerie-exemple.ch"] : ["boulangerie-exemple.ch", "atelier-exemple.ch"];
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: new RegExp(`${site.replaceAll(".", "\\.")}$`) }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText(site);
  await expect(page.getByRole("list", { name: "Site links" }).getByRole("button", { name: "Copy SSH command" })).toBeVisible();

  // Words no other test searches for: the notes stay on the shared data after this test.
  const word = `contrat${browserName}`;
  await page.getByRole("tab", { name: "Notes" }).click();
  await page.getByRole("button", { name: /^(Add notes|Edit)$/ }).click();
  await page.getByRole("textbox", { name: `Notes of ${site}` }).fill(`**Client**: ${word}\n\n### Contract\n\n- ${word} until 2027`);
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.getByRole("heading", { name: "Contract" })).toBeVisible();
  await expect(page.getByRole("tabpanel")).toContainText(`${word} until 2027`);

  // The search reads the notes; the row says why it is there.
  await page.getByRole("button", { name: "Search" }).click();
  await page.getByRole("searchbox", { name: "Search" }).fill(word);
  await expect(tree.getByRole("treeitem", { name: new RegExp(site.replaceAll(".", "\\.")) })).toContainText("in the notes");
  await expect(tree.getByRole("treeitem", { name: new RegExp(other.replaceAll(".", "\\.")) })).toHaveCount(0);
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");

  // The host's table: sorted by a header, filtered by a count.
  await tree.getByRole("treeitem", { name: /^Shared hosting/ }).click();
  const table = page.getByRole("table");
  const updates = table.getByRole("button", { name: "Updates" });
  await updates.click();
  await expect(table.getByRole("columnheader", { name: "Updates" })).toHaveAttribute("aria-sort", "descending");
  const summary = page.getByRole("list", { name: "Summary" });
  const filter = summary.getByRole("button").first();
  await filter.click();
  await expect(filter).toHaveAttribute("aria-pressed", "true");
  await page.getByRole("button", { name: "Show all" }).click();
  await expect(filter).toHaveAttribute("aria-pressed", "false");
});
