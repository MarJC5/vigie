import { expect, test } from "@playwright/test";

// The seeded server: read only, shared by both browsers. No host of the seed answers over SSH.
const seeded = "http://localhost:34115/";

test("a site's copies have their tab, and a copy whose sizes cannot be read says so", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  // Copy is the action of the Copies tab only; the History tab has no copies.
  await expect(page.getByRole("button", { name: "Copy", exact: true })).toHaveCount(0);
  await page.getByRole("tab", { name: "History" }).click();
  await expect(page.getByRole("heading", { name: "Copies on this computer" })).toHaveCount(0);
  await page.getByRole("tab", { name: "Copies" }).click();
  await expect(page.getByRole("heading", { name: "Copies on this computer" })).toBeVisible();
  await expect(page.getByText("No copy of this site yet.")).toBeVisible();

  await page.getByRole("button", { name: "Copy", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Copy boulangerie-exemple.ch to this computer" });
  await expect(dialog.getByRole("alert")).toBeVisible({ timeout: 30_000 });
  await expect(dialog.getByRole("button", { name: "Copy" })).toBeDisabled();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
});

test("the folder of the copies and the size of the archives are in Settings, Data", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Data" }).click();
  await expect(page.getByRole("heading", { name: "Copies on this computer" })).toBeVisible();
  await expect(page.getByText(/\/Documents\/Vigie$/)).toBeVisible();
  await expect(page.getByRole("combobox", { name: "Size of the archives" })).toHaveValue("500");
});

test("the site's menu offers Copy to this computer, from any tab", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Copy to this computer" }).click();
  const dialog = page.getByRole("dialog", { name: "Copy boulangerie-exemple.ch to this computer" });
  await expect(dialog).toBeVisible();
  await dialog.getByRole("button", { name: "Cancel" }).click();
});

test("the items of a selection offer Copy, the content only", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  await tree.getByRole("treeitem", { name: /atelier-exemple\.ch/ }).click({ modifiers: ["ControlOrMeta"] });
  await page.getByRole("button", { name: "Copy", exact: true }).click();
  const dialog = page.getByRole("dialog", { name: "Copy 2 sites to this computer" });
  await expect(dialog.getByRole("button", { name: "Copy the 2 sites" })).toBeEnabled();
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).toHaveCount(0);
});
