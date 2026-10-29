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
  await expect(page.getByRole("main")).toContainText("Multisite network");
  await expect(page.getByRole("main")).toContainText("Clients/Shared hosting/reseau-ecoles.ch");
});

test("the window has one of each landmark", async ({ page }) => {
  await page.goto(seeded);
  await expect(page.getByRole("navigation", { name: "Navigation" })).toHaveCount(1);
  await expect(page.getByRole("complementary", { name: "Site manager" })).toHaveCount(1);
  await expect(page.getByRole("main")).toHaveCount(1);
});

test("the tree works from the keyboard", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /^Clients/ }).focus();
  await page.keyboard.press("ArrowDown");
  await page.keyboard.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Shared hosting");
  await expect(page.getByRole("complementary")).toContainText("Shared hosting · 3 sites");
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
