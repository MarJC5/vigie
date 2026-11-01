import { expect, test } from "@playwright/test";

// The plugins and themes from the latest scans (ADR-0024): a site's tab and its filter, then a
// host's tab, a line opened to its sites, a site's name opening that site. Read-only on the shared
// data.
const seeded = "http://localhost:34115/";

test("a site's plugins, then the plugins across a host's sites", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /boulangerie-exemple\.ch$/ }).click();
  await page.getByRole("button", { name: "Show the plugins" }).click();
  await expect(page.getByRole("tab", { name: "Plugins" })).toHaveAttribute("aria-selected", "true");
  const plugins = page.getByRole("table", { name: "Plugins" });
  const akismet = plugins.getByRole("row", { name: /Akismet Anti-spam/ });
  await expect(akismet).toContainText("5.3");
  await expect(akismet).toContainText("→ 5.7.2");
  await expect(akismet).toContainText("Fictional stored cross-site scripting");

  // The filter: an active plugin with an update, none inactive.
  const filters = page.getByRole("group", { name: "Show" });
  await filters.getByRole("button", { name: "Inactive" }).click();
  await expect(page.getByRole("tabpanel")).toContainText("Nothing to show with this filter.");
  await filters.getByRole("button", { name: "With an update" }).click();
  await expect(akismet).toBeVisible();

  // The host: the network never scanned is left out; Akismet runs in two versions.
  await tree.getByRole("treeitem", { name: /^Shared hosting/ }).click();
  await page.getByRole("tablist", { name: "Host tabs" }).getByRole("tab", { name: "Plugins" }).click();
  await expect(page.getByRole("tabpanel")).toContainText("1 site left out, not known yet: reseau-ecoles.ch.");
  const across = page.getByRole("table", { name: "Plugins and themes across the sites" });
  const line = across.getByRole("row", { name: /Akismet Anti-spam/ }).first();
  await expect(line).toContainText("2 of 2");
  await expect(line).toContainText("2 versions");
  await expect(line).toContainText("1 site");

  const open = across.getByRole("button", { name: "Sites running Akismet Anti-spam" });
  await open.click();
  await expect(open).toHaveAttribute("aria-expanded", "true");
  // The oldest version first.
  await expect(across.getByRole("button", { name: /-exemple\.ch$/ })).toHaveText(["boulangerie-exemple.ch", "atelier-exemple.ch"]);

  // The search keeps what matches.
  await page.getByRole("searchbox", { name: "Search plugins and themes" }).fill("woocommerce");
  await expect(page.getByRole("tabpanel")).toContainText("Nothing matches this search.");
  await page.getByRole("searchbox", { name: "Search plugins and themes" }).fill("akis");
  await across.getByRole("button", { name: "atelier-exemple.ch" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("atelier-exemple.ch");
});
