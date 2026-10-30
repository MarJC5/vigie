import { expect, test } from "@playwright/test";

// The background settings on an empty Vigie (no menu bar nor Dock in the server build): kept once
// chosen, start at login not offered where the system has none, the next scan under the site list.
// One server per browser.
test("the background settings are kept, and the next automatic scan shows under the site list", async ({ page }, info) => {
  const url = info.project.name === "webkit" ? "http://localhost:34128/" : "http://localhost:34127/";
  await page.goto(url);
  const rail = page.getByRole("navigation", { name: "Navigation" });
  await rail.getByRole("link", { name: "Settings" }).click();

  const keep = page.getByRole("switch", { name: "Keep Vigie running" });
  await expect(keep).toHaveAttribute("aria-checked", "false");
  await expect(page.getByRole("switch", { name: "Open at login" })).toHaveCount(0);
  await keep.click();
  await expect(keep).toHaveAttribute("aria-checked", "true");
  await page.getByRole("combobox", { name: "Automatic scans" }).selectOption("6");

  await page.reload();
  await rail.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByRole("switch", { name: "Keep Vigie running" })).toHaveAttribute("aria-checked", "true");
  await expect(page.getByRole("combobox", { name: "Automatic scans" })).toHaveValue("6");

  await rail.getByRole("link", { name: "Sites" }).click();
  await expect(page.getByText(/^Next scan: (\d\d\.\d\d\.\d{4} )?\d\d:\d\d$/)).toBeVisible();
});
