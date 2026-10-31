import { expect, test } from "@playwright/test";

// The help of the built window: getting started, then the short topics of docs/help, bundled with it.
const seeded = "http://localhost:34115/";

test("the Help page opens from the rail, shows getting started and the short topics", async ({ page }) => {
  await page.goto(seeded);
  const rail = page.getByRole("navigation", { name: "Navigation" });
  await rail.getByRole("link", { name: "Help" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Help" })).toBeVisible();
  await expect(page.getByRole("tab", { name: "Getting started", selected: true })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Your first host" })).toBeVisible();
  await expect(page.getByText("“The server did not answer.”")).toBeVisible();
  // One of each landmark: the window's, then the page's.
  await expect(page.getByRole("main")).toHaveCount(1);
  await expect(page.getByRole("complementary", { name: "Shortcuts" })).toHaveCount(1);

  // A short topic, ending with its full doc on GitHub.
  await page.getByRole("tab", { name: "Background" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Keep Vigie running in the background" })).toBeVisible();
  await expect(page.getByRole("navigation", { name: "On this page" })).toHaveCount(0);
  await expect(page.getByRole("link", { name: "Vigie in the background, and scans with Vigie closed" })).toBeVisible();
  await page.getByRole("tab", { name: "Updating" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Update a site, and roll back" })).toBeInViewport();
});
