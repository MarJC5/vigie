import { expect, test } from "@playwright/test";

// A test add-on on the PATH of an empty Vigie (the fake of package addons, as vigie-good): found,
// run from its page with its setting, its export previewed, then added. One server per browser.
test("an installed add-on runs from its page, and what it wrote is added after the preview", async ({ page }, info) => {
  await page.goto(info.project.name === "webkit" ? "http://localhost:34126/" : "http://localhost:34125/");
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Add-ons" }).click();
  await page.getByRole("tab", { name: "Installed" }).click();
  await page.getByRole("button", { name: "Open Fake good" }).click();
  await expect(page.getByRole("heading", { level: 1, name: "Fake good" })).toBeVisible();

  const names = page.getByRole("textbox", { name: "Names" });
  await expect(names).toHaveValue("shop");
  await names.fill("shop,blog");
  await page.getByRole("button", { name: "Run the import" }).click();

  await expect(page.getByRole("heading", { level: 1, name: "Import from Fake good" })).toBeVisible();
  await expect(page.getByText("Written by vigie-good 0.0.1. Nothing is added until you confirm.")).toBeVisible();
  // What the add-on said, word for word.
  await expect(page.getByText("2 hosts.")).toBeVisible();
  await page.getByRole("button", { name: "Add 2 items" }).click();

  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Sites" }).click();
  const tree = page.getByRole("tree", { name: "Site manager" });
  await expect(tree.getByRole("treeitem", { name: /^shop/ })).toBeVisible();
  await expect(tree.getByRole("treeitem", { name: /^blog/ })).toBeVisible();
});
