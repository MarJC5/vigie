import { expect, test } from "@playwright/test";

// What changed since the previous scan (ADR-0027), on the seed's two complete scans of
// boulangerie-exemple.ch two days apart: on the site's Overview and in the journal.
const seeded = "http://localhost:34115/";
const changes = [
  "Administrator removed: stagiaire",
  "Plugin removed: Hello Dolly 1.7.2",
  "WordPress 7.1.2 → 7.1.3, outside Vigie",
  "New finding: Files can be edited from the dashboard",
];

test("a site's Overview says what changed since its previous scan", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /boulangerie-exemple\.ch$/ }).click();
  await expect(page.getByRole("heading", { level: 2, name: /^Since the previous scan/ })).toBeVisible();
  const list = page.getByRole("list", { name: "Since the previous scan" });
  await expect(list.getByRole("listitem")).toHaveText(changes);
  await expect(list.getByRole("img", { name: "Worth a look" })).toHaveCount(1);

  // A site scanned once has nothing to compare.
  await tree.getByRole("treeitem", { name: /atelier-exemple\.ch$/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("atelier-exemple.ch");
  await expect(page.getByRole("heading", { level: 2, name: /^Since the previous scan/ })).toHaveCount(0);
});

test("the journal says under a scan what changed on its site, the name opening its page", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Activity" }).click();
  const list = page.getByRole("list", { name: "Changes of boulangerie-exemple.ch" });
  await expect(list).toHaveCount(1);
  await expect(list.getByRole("listitem")).toHaveText(changes);
  await list.locator("..").getByRole("button", { name: "boulangerie-exemple.ch", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
});
