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

// Tags (ADR-0025): written on two sites, found by the search, a tag's page opened, renamed onto
// another tag, then removed from every site.
test("a tag across sites: its page, renamed onto another, removed", async () => {
  const addTags = async (site: RegExp, ...typed: string[]) => {
    await item(site).click();
    await page.getByRole("button", { name: "Add a tag" }).click();
    for (const text of typed) {
      await page.keyboard.type(text);
      await page.keyboard.press("Enter");
      await expect(page.getByRole("combobox", { name: "New tag" })).toHaveValue("");
    }
    await page.keyboard.press("Escape");
  };
  await addTags(/atelier\.ch$/, "Contrat");
  // Typed in another case: the spelling already used is kept.
  await addTags(/boulangerie-exemple\.ch$/, "contrat", "Dupont");
  await expect(page.getByRole("list", { name: "Tags" })).toContainText("Contrat");
  await expect(page.getByRole("list", { name: "Tags" })).not.toContainText("contrat");

  // The search reads the tags.
  await page.getByRole("button", { name: "Search" }).click();
  await page.getByRole("searchbox", { name: "Search" }).fill("dupont");
  await expect(item(/boulangerie-exemple\.ch/)).toContainText("in the tags");
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");
  await page.getByRole("searchbox", { name: "Search" }).press("Escape");

  // The tag's page: its two sites, where each is.
  await item(/^Contrat/).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Contrat");
  await expect(page.getByRole("table")).toContainText("Host");
  await expect(page.getByRole("table").getByRole("row")).toHaveCount(3);
  await expect(page.getByRole("complementary")).toContainText("Contrat · 2 sites");

  // Renamed onto Dupont: both become one, and the page follows.
  await page.getByRole("button", { name: "More actions" }).click();
  await page.getByRole("menuitem", { name: "Rename the tag…" }).click();
  const rename = page.getByRole("dialog", { name: "Rename “Contrat”" });
  await rename.getByRole("textbox", { name: "Name" }).fill("dupont");
  await expect(rename.getByRole("note")).toContainText("Both become one tag, on 2 sites.");
  await rename.getByRole("button", { name: "Rename" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Dupont");
  await expect(item(/^Dupont/)).toContainText("2");
  await expect(item(/^Contrat/)).toHaveCount(0);

  // Removed from every site: the group goes with its last tag.
  await page.getByRole("button", { name: "More actions" }).click();
  await page.getByRole("menuitem", { name: "Remove from every site…" }).click();
  await page.getByRole("dialog", { name: "Remove “Dupont” from every site?" }).getByRole("button", { name: "Remove the tag" }).click();
  await expect(item(/^Dupont/)).toHaveCount(0);
  await expect(item(/^Tags/)).toHaveCount(0);
});

test("a site's backups: listed in its History tab, those past 30 days removed after a confirmation", async () => {
  await item(/boulangerie-exemple\.ch/).click();
  await page.getByRole("tab", { name: "History" }).click();
  const panel = page.getByRole("tabpanel");
  await expect(panel.getByRole("heading", { name: "Backups on the server" })).toBeVisible();
  await expect(panel.getByText("1 backup over 30 days old, 45.1 MB")).toBeVisible();
  await panel.getByRole("button", { name: "Remove…" }).click();
  const dialog = page.getByRole("dialog", { name: "Remove 1 backup of boulangerie-exemple.ch?" });
  await dialog.getByRole("button", { name: "Remove the backup" }).click();
  // The seed's servers do not exist: the footer says the server could not be reached.
  await expect(page.getByRole("complementary").getByRole("alert")).toBeVisible();
});

test("a site deleted with its backups, its server unreachable: the command that removes them", async () => {
  await item(/boulangerie-exemple\.ch/).click({ button: "right" });
  await page.getByRole("menuitem", { name: "Delete the site…" }).click();
  const dialog = page.getByRole("dialog", { name: 'Delete "boulangerie-exemple.ch"?' });
  await expect(dialog.getByRole("checkbox", { name: /Also remove its backups on the server/ })).toBeChecked();
  await expect(dialog.getByText(/^2 backups, 93 MB, in ~\/\.vigie\/backups\./)).toBeVisible();
  await dialog.getByRole("button", { name: "Delete the site" }).click();
  const left = page.getByRole("dialog", { name: "“boulangerie-exemple.ch” is deleted from Vigie" });
  await expect(left.getByText(/^rm -rf ~\/\.vigie\/backups\/[0-9a-f-]{36}$/)).toBeVisible();
  await left.getByRole("button", { name: "Close" }).click();
  await expect(item(/boulangerie-exemple\.ch/)).toHaveCount(0);
});

test("how long backups stay on the servers is kept in the settings", async () => {
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Data" }).click();
  const keep = page.getByRole("combobox", { name: "Backups before updates" });
  await expect(keep).toHaveValue("30");
  await keep.selectOption({ label: "90 days" });
  await page.reload();
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Data" }).click();
  await expect(page.getByRole("combobox", { name: "Backups before updates" })).toHaveValue("90");
});

// Several items selected (ADR-0034): their page, a drag that carries every host, one delete for all.
test("several items: their page, then a drag moves every host at once", async () => {
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Sites" }).click();
  await item(/^Personal/).focus();
  await page.keyboard.press("ArrowRight");
  await item(/Shared hosting/).click();
  await item(/Managed VPS/).click({ modifiers: ["ControlOrMeta"] });
  await expect(tree()).toHaveAttribute("aria-multiselectable", "true");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("2 items selected");
  await expect(page.getByRole("complementary")).toContainText("2 selected · 2 sites");
  await expect(page.getByRole("table", { name: "Sites of the selection" }).getByRole("row")).toHaveCount(3);

  // Shared hosting is already in Clients: the drag of the VPS brings it there too.
  await item(/Managed VPS/).dragTo(item(/^Clients/));
  await item(/^Clients/).focus();
  await page.keyboard.press("ArrowLeft");
  await expect(item(/Managed VPS/)).toHaveCount(0);
  await expect(item(/Shared hosting/)).toHaveCount(0);
  await page.keyboard.press("ArrowRight");
});

test("several items deleted at once: what goes, and the folder that stays", async () => {
  await item(/^Personal/).click();
  await item(/Managed VPS/).click({ modifiers: ["ControlOrMeta"] });
  await item(/^Clients/).click({ modifiers: ["ControlOrMeta"] });
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("3 items selected");
  await page.keyboard.press("ControlOrMeta+Backspace");
  const dialog = page.getByRole("dialog", { name: "Delete 2 items?" });
  await expect(dialog.getByRole("listitem")).toHaveText(["Managed VPS0 sites", "Personalempty"]);
  await expect(dialog.getByRole("note")).toHaveText("“Clients” stays: only an empty folder can be deleted.");
  await dialog.getByRole("button", { name: "Delete the 2 items" }).click();
  await expect(item(/Managed VPS/)).toHaveCount(0);
  await expect(item(/^Personal/)).toHaveCount(0);
  await expect(item(/^Clients/)).toBeVisible();
});
