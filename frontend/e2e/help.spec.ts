import { expect, test } from "@playwright/test";

// The help of the built window: getting started, then the docs of the repository, bundled with it.
const seeded = "http://localhost:34115/";

test("the Help page opens from the rail, shows the docs and moves between them", async ({ page }) => {
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

  // A doc, with its sections beside it.
  await page.getByRole("tab", { name: "Background" }).click();
  await expect(page.getByRole("heading", { level: 2, name: "Vigie in the background" })).toBeVisible();
  const onPage = page.getByRole("navigation", { name: "On this page" });
  await expect(onPage).toHaveCount(1);
  await onPage.getByRole("link", { name: "Scans with Vigie closed" }).click();
  await expect(page.getByRole("heading", { level: 3, name: "Scans with Vigie closed" })).toBeInViewport();

  // A link to another doc opens its tab, at its top.
  await page.getByRole("link", { name: "security.md" }).click();
  await expect(page.getByRole("tab", { name: "Security", selected: true })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: /^Security scans/ })).toBeInViewport();
});
