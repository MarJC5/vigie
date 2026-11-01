import { expect, test } from "@playwright/test";

// The window loads only what the app embeds (ADR-0022): every section is opened, and any request,
// script or style the policy refuses fails the test.
const seeded = "http://localhost:34115/";

test("every section of the window works under its Content Security Policy", async ({ page }) => {
  const refused: string[] = [];
  page.on("console", (message) => {
    if (message.text().startsWith("Refused by the policy:")) refused.push(message.text());
  });
  await page.addInitScript(() => {
    document.addEventListener("securitypolicyviolation", (event) => {
      const element = event.target instanceof Element ? event.target : null;
      // WebKit styles the inside of a native select itself, and its own policy check refuses it:
      // nothing the page wrote (the select carries no style attribute), and the list still shows.
      if (element?.tagName === "SELECT" && !element.hasAttribute("style") && event.violatedDirective === "style-src-attr") return;
      // What was refused, and on which element, so that a failure says where to look.
      console.error(`Refused by the policy: ${event.violatedDirective} ${event.blockedURI} ${element?.outerHTML.slice(0, 160) ?? ""}`);
    });
  });
  await page.goto(seeded);
  await expect(page.locator('meta[http-equiv="Content-Security-Policy"]')).toHaveCount(1);

  // A site and its tabs.
  await page.getByRole("tree", { name: "Site manager" }).getByRole("treeitem", { name: /boulangerie-exemple\.ch$/ }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("boulangerie-exemple.ch");
  for (const tab of ["Updates", "Security", "History", "Overview"]) {
    await page.getByRole("tab", { name: tab }).click();
    await expect(page.getByRole("tab", { name: tab, selected: true })).toBeVisible();
  }

  // The pages of the rail, each of their tabs.
  const rail = page.getByRole("navigation", { name: "Navigation" });
  const pages: Array<[string, string[]]> = [
    ["Activity", []],
    ["Add-ons", ["Library", "Installed"]],
    ["Settings", ["General", "Lookups outside", "Data", "About"]],
    ["Help", ["Getting started", "Updating", "Site status", "Read and sent", "Import, export", "Background"]],
  ];
  for (const [name, tabs] of pages) {
    await rail.getByRole("link", { name }).click();
    await expect(page.getByRole("heading", { level: 1, name })).toBeVisible();
    for (const tab of tabs) {
      await page.getByRole("tab", { name: tab }).click();
      await expect(page.getByRole("tab", { name: tab, selected: true })).toBeVisible();
    }
  }

  expect(refused).toEqual([]);
});
