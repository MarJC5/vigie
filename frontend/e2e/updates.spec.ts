import { expect, test } from "@playwright/test";

// The seeded server: read only, shared by both browsers. No host of the seed answers over SSH.
const seeded = "http://localhost:34115/";

test("a site lists its updates and their history, the rollback offered on the newest", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  await page.getByRole("tab", { name: "History" }).click();
  const history = page.getByRole("list", { name: "History of updates" });
  const newest = history.getByRole("button", { expanded: true });
  await expect(newest).toContainText("Update of 2 items");
  await expect(newest).toContainText("Failed");
  await expect(history).toContainText("woocommerce 9.8.1 → 10.2.0");
  await expect(history).toContainText("answers 500 (critical-error)");
  await expect(history.getByRole("button", { name: "Roll back" })).toBeEnabled();
  // The older update opens on a click.
  const older = history.getByRole("button", { name: /Update of 1 item/ });
  await expect(older).toHaveAttribute("aria-expanded", "false");
  await older.click();
  await expect(older).toHaveAttribute("aria-expanded", "true");
  await expect(history.getByRole("button", { name: "Roll back" })).toHaveCount(1);
});

test("an update whose preview cannot be read says so, and starts nothing", async ({ page }) => {
  await page.goto(seeded);
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  await page.getByRole("tab", { name: /Updates/ }).click();
  await expect(page.getByText("3 of 3 selected")).toBeVisible();
  await page.getByRole("button", { name: "Update (3)" }).click();

  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Update boulangerie-exemple.ch");
  await expect(page.getByRole("alert")).toContainText("This update cannot start", { timeout: 30_000 });
  await expect(page.getByRole("button", { name: "Back up and update" })).toBeDisabled();
  await page.getByRole("button", { name: "Cancel" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
  await expect(page.getByRole("complementary", { name: "Site manager" })).not.toContainText("updating");
});
