import { expect, type Page, test } from "@playwright/test";

// The editing tests change their data, in order: each browser has its own server.
test.describe.configure({ mode: "serial" });

let page: Page;
const tree = () => page.getByRole("tree", { name: "Site manager" });
const item = (name: RegExp) => tree().getByRole("treeitem", { name });

test.beforeAll(async ({ browser }, info) => {
  page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto(info.project.name === "webkit" ? "http://localhost:34118/" : "http://localhost:34117/");
});

test.afterAll(async () => {
  await page.close();
});

test("a folder made from a folder's menu is named in place", async () => {
  await item(/^Clients/).click({ button: "right" });
  await page.getByRole("menu", { name: "Actions on Clients" }).getByRole("menuitem", { name: "New folder" }).click();
  const field = page.getByRole("textbox", { name: "Name of New folder" });
  await expect(field).toBeFocused();
  await field.fill("Valais");
  await field.press("Enter");
  await expect(item(/^Valais/)).toHaveAttribute("aria-level", "2");
});

test("a site is renamed with Enter", async () => {
  await item(/atelier-exemple\.ch/).click();
  await page.keyboard.press("Enter");
  const field = page.getByRole("textbox", { name: "Name of atelier-exemple.ch" });
  await field.fill("atelier.ch");
  await field.press("Enter");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("atelier.ch");
});

test("a name already used is refused, and the footer says why", async () => {
  await item(/^Valais/).click();
  await page.keyboard.press("Enter");
  const field = page.getByRole("textbox", { name: "Name of Valais" });
  await field.fill("Shared hosting");
  await field.press("Enter");
  await expect(page.getByRole("complementary").getByRole("alert")).toHaveText("This name is already used here.");
  await expect(item(/^Valais/)).toBeVisible();
});

test("a host is moved from its menu", async () => {
  await item(/Managed VPS/).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Move to…" }).click();
  const dialog = page.getByRole("dialog", { name: 'Move "Managed VPS"' });
  await dialog.getByRole("radio", { name: /Clients/ }).check();
  await dialog.getByRole("button", { name: "Move" }).click();
  await expect(dialog).toHaveCount(0);
  await expect(item(/Managed VPS/)).toHaveAttribute("aria-level", "2");
});

// WebKit does not focus a clicked radio: Escape must close the dialog all the same.
test("Escape closes a dialog after a choice", async () => {
  await item(/Managed VPS/).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Move to…" }).click();
  await page.getByRole("radio", { name: /Personal/ }).check();
  await page.keyboard.press("Escape");
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(page.getByRole("navigation", { name: "Navigation" })).not.toHaveAttribute("inert");
});

test("a host is dragged back into a folder", async () => {
  await item(/Managed VPS/).dragTo(item(/^Personal/));
  await expect(item(/Managed VPS/)).toHaveAttribute("aria-level", "2");
  await item(/^Personal/).click();
  await expect(page.getByRole("complementary")).toContainText("Personal · 0 sites");
  await expect(page.getByRole("main")).toContainText("No site in this folder yet.");
});

test("a site takes a color of its own from its menu", async () => {
  await item(/boulangerie-exemple\.ch/).click({ button: "right" });
  await page.getByRole("menuitemradio", { name: "Green" }).click();
  await item(/boulangerie-exemple\.ch/).click({ button: "right" });
  await expect(page.getByRole("menuitemradio", { name: "Green" })).toHaveAttribute("aria-checked", "true");
  await page.keyboard.press("Escape");
});

test("an empty folder is deleted after a confirmation", async () => {
  await item(/^Valais/).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Delete the folder…" }).click();
  await page.getByRole("dialog", { name: 'Delete the folder "Valais"?' }).getByRole("button", { name: "Delete the folder" }).click();
  await expect(item(/^Valais/)).toHaveCount(0);
});

test("the site list stays as the user left it: across pages, then after a reload", async () => {
  const rail = page.getByRole("navigation", { name: "Navigation" });
  await item(/^Personal/).focus();
  await page.keyboard.press("ArrowLeft");
  await expect(item(/^Personal/)).toHaveAttribute("aria-expanded", "false");
  await expect(item(/^Managed VPS/)).toHaveCount(0);

  await rail.getByRole("link", { name: "Settings" }).click();
  await rail.getByRole("link", { name: "Sites" }).click();
  await expect(item(/^Personal/)).toHaveAttribute("aria-expanded", "false");

  await page.getByRole("button", { name: "Hide the site list" }).click();
  await page.reload();
  await expect(tree()).toHaveCount(0);
  await page.getByRole("button", { name: "Show the site list" }).click();
  await expect(item(/^Personal/)).toHaveAttribute("aria-expanded", "false");
  await expect(item(/^Clients/)).toHaveAttribute("aria-expanded", "true");
});
