import { expect, test } from "@playwright/test";

// The scan's complements (ADR-0028), on the seed's latest scan of boulangerie-exemple.ch: PHP on the
// web as its dashboard noted it, the admin email and the one waiting, debug.log written by WordPress.
const seeded = "http://localhost:34115/";

test("a site's Overview says PHP on the web, the admin email and debug.log", async ({ page }) => {
  await page.goto(seeded);
  const tree = page.getByRole("tree", { name: "Site manager" });
  await tree.getByRole("treeitem", { name: /boulangerie-exemple\.ch$/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
  const row = (label: string) => page.locator("dt", { hasText: new RegExp(`^${label}$`) }).locator("..");
  await expect(row("PHP on the command line")).toContainText("8.4.27");
  await expect(row("PHP on the web")).toContainText("8.2.29");
  await expect(row("PHP on the web")).toContainText(/seen by the dashboard on \d{2}\.\d{2}\.\d{4}/);
  await expect(row("Admin email")).toContainText("contact@boulangerie-exemple.ch");
  await expect(row("Admin email")).toContainText("Waiting for confirmation: marie@boulangerie-exemple.ch");
  await expect(row("debug.log")).toContainText("38.2 MB");
  await expect(row("debug.log")).toContainText("written by WordPress to wp-content/debug.log");

  // A site scanned before: nothing of it, rather than a guess.
  await tree.getByRole("treeitem", { name: /atelier-exemple\.ch$/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("atelier-exemple.ch");
  await expect(page.locator("dt", { hasText: /^PHP on the web$/ })).toHaveCount(0);
});
