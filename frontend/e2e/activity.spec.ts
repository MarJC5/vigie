import { expect, test } from "@playwright/test";

// The journal of the seed (its scans and updates), read from the Go side: by day, a site's name
// opens its page, the filter keeps one site's entries.
const seeded = "http://localhost:34115/";

test("the Activity page tells what happened, by day, and opens a site's page", async ({ page }) => {
  await page.goto(seeded);
  const rail = page.getByRole("navigation", { name: "Navigation" });
  await rail.getByRole("link", { name: "Activity" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Activity" })).toBeVisible();
  await expect(page.getByRole("heading", { level: 2, name: "Today" })).toBeVisible();
  // Today's update failed; the one of a week ago completed and says nothing more.
  const updates = page.getByRole("listitem").filter({ hasText: "Update of Clients/Shared hosting/boulangerie-exemple.ch" });
  await expect(updates).toHaveCount(2);
  await expect(updates.first()).toContainText("failed");
  await expect(updates.first()).toContainText("1 of 2");

  // The filter keeps one site's entries.
  await page.getByRole("combobox", { name: "Show" }).selectOption({ label: "Clients/Shared hosting/atelier-exemple.ch" });
  await expect(updates).toHaveCount(0);
  await page.getByRole("combobox", { name: "Show" }).selectOption({ label: "Every site" });
  await expect(updates).toHaveCount(2);

  await updates.first().getByRole("button", { name: "Clients/Shared hosting/boulangerie-exemple.ch" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
});
