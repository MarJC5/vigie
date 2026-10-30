import { expect, test } from "@playwright/test";

const seeded = "http://localhost:34115/";
const empty = "http://localhost:34116/";

test("the site manager comes from the Go side, and selecting an item shows its detail", async ({ page }) => {
  await page.goto(seeded);
  await expect(page.locator("html")).toHaveAttribute("lang", "en");
  const tree = page.getByRole("tree", { name: "Site manager" });
  await expect(tree.getByRole("treeitem")).toHaveCount(7);
  await expect(tree.getByRole("treeitem", { name: /reseau-ecoles\.ch/ })).toContainText("2 subsites");

  await tree.getByRole("treeitem", { name: /reseau-ecoles\.ch/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("reseau-ecoles.ch");
  await expect(page.getByRole("tabpanel")).toContainText("Multisite network");
  await expect(page.getByRole("tabpanel")).toContainText("This site has not been scanned yet.");
});

test("the window has one of each landmark", async ({ page }) => {
  await page.goto(seeded);
  await expect(page.getByRole("navigation", { name: "Navigation" })).toHaveCount(1);
  await expect(page.getByRole("complementary", { name: "Site manager" })).toHaveCount(1);
  await expect(page.getByRole("main")).toHaveCount(1);
});

test("the window opens on the first host, and the tree works from the keyboard", async ({ page }) => {
  await page.goto(seeded);
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Shared hosting");
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /^Shared hosting/ }).focus();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("atelier-exemple.ch");
  await page.keyboard.press("ArrowUp");
  await page.keyboard.press("ArrowUp");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Clients");
  await expect(page.getByRole("complementary")).toContainText("Clients · 3 sites");
});

test("a first launch invites to add a host, in the stored language", async ({ page }) => {
  await page.goto(empty);
  await expect(page.locator("html")).toHaveAttribute("lang", "fr");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Ajoutez votre premier hébergement");
  await expect(page.getByText("Aucun hébergement pour le moment.")).toBeVisible();
});

test("the rail opens the add-a-host wizard, which closes back to the tree", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("button", { name: "Add a host" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Add a host");
  await expect(page.getByRole("main")).toHaveCount(1);
  await expect(page.getByRole("button", { name: "Continue" })).toBeDisabled();
  await page.getByRole("button", { name: "Close" }).click();
  await expect(page.getByRole("tree", { name: "Site manager" })).toBeVisible();
});

test("the tree shows the latest scans, a site its overview and its updates", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await expect(tree.getByRole("img", { name: "Needs attention" })).toHaveCount(1);
  await expect(tree.getByRole("img", { name: "Up to date" })).toHaveCount(1);

  await tree.getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
  // The seed's scan is 3 minutes old when its server starts; the servers of a slow run start earlier.
  await expect(page.getByText(/^Last scan: \d+ minutes ago$/)).toBeVisible();
  await expect(page.getByRole("tabpanel")).toContainText("7.1.3 · 7.1.4 available");
  await page.getByRole("tab", { name: /Updates/ }).click();
  await expect(page.getByRole("tabpanel")).toContainText("5.3 → 5.7.2");
});

test("a host shows the table of its sites", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /^Shared hosting/ }).click();
  await expect(page.getByText("alias shared · wp")).toBeVisible();
  const table = page.getByRole("table", { name: "Sites of Shared hosting" });
  await expect(table.getByRole("row")).toHaveCount(4);
  await expect(page.getByRole("list", { name: "Summary" })).toContainText("Not scanned1");
});

test("the search keeps what matches", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("button", { name: "Search" }).click();
  await page.getByRole("searchbox", { name: "Search" }).fill("boul");
  await expect(page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem")).toHaveCount(3);
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");
  await expect(page.getByRole("searchbox")).toHaveCount(0);
  await expect(page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem")).toHaveCount(7);
});

test("the rail opens the settings, which list what each outside service receives", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Settings");
  await expect(page.getByRole("tree")).toHaveCount(0);
  await expect(page.getByRole("main")).toHaveCount(1);
  await page.getByRole("tab", { name: "Lookups outside" }).click();
  await expect(page.getByRole("switch")).toHaveCount(4);
  await expect(page.getByRole("tabpanel")).toContainText("Receives: plugin slugs · answer kept 1 day");
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Sites" }).click();
  await expect(page.getByRole("tree", { name: "Site manager" })).toBeVisible();
});

test("a cancelled scan keeps the results of the complete one before", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /atelier-exemple\.ch/ }).click();
  await expect(page.getByRole("tabpanel")).toContainText("The latest scan did not complete");
  await expect(page.getByRole("tabpanel")).toContainText("Scan cancelled. These results are from the scan of");
  await expect(page.getByRole("tabpanel")).toContainText("7.1.3");
  await expect(tree.getByRole("treeitem", { name: /atelier-exemple\.ch/ }).getByRole("img", { name: "Up to date" })).toBeVisible();
});

test("the appearance chosen in the settings applies at once, the system's when none is", async ({ page }) => {
  await page.goto(empty);
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Réglages" }).click();
  // Both browsers share this server: the test does not rely on the choice it finds.
  const appearance = page.getByRole("combobox", { name: "Apparence" });
  await appearance.selectOption("light");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "light");
  await appearance.selectOption("system");
  await expect(page.locator("html")).not.toHaveAttribute("data-theme");
});
