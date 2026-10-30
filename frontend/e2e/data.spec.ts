import { copyFileSync, readdirSync, readFileSync, statSync, writeFileSync } from "node:fs";
import { join } from "node:path";

import { expect, type Page, test } from "@playwright/test";

// The seed exported with its history, then imported into an empty Vigie: each browser has its two
// servers, sharing a folder for the files (the server build has no file dialogs).
test.describe.configure({ mode: "serial" });

let seeded: Page;
let empty: Page;
let files: string;

const tree = (page: Page) => page.getByRole("tree", { name: "Site manager" });
const rail = (page: Page) => page.getByRole("navigation", { name: "Navigation" });
const dataTab = async (page: Page) => {
  await rail(page).getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Data" }).click();
};

test.beforeAll(async ({ browser }, info) => {
  const webkit = info.project.name === "webkit";
  files = join(process.env.VIGIE_E2E_FILES!, info.project.name);
  seeded = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  empty = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await seeded.goto(webkit ? "http://localhost:34123/" : "http://localhost:34121/");
  await empty.goto(webkit ? "http://localhost:34124/" : "http://localhost:34122/");
});

test.afterAll(async () => {
  await seeded.close();
  await empty.close();
});

test("Export… in the menu of a host chooses that host", async () => {
  await tree(seeded).getByRole("treeitem", { name: /Shared hosting/ }).click({ button: "right" });
  await seeded.getByRole("menuitem", { name: "Export…" }).click();
  const dialog = seeded.getByRole("dialog", { name: "Export the site manager" });
  await expect(dialog.getByRole("radio", { name: /A folder or a host/ })).toBeChecked();
  await expect(dialog.getByRole("combobox", { name: "A folder or a host" })).toHaveValue(/.+/);
  await expect(dialog).toContainText("1 host, 3 sites");
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(seeded.getByRole("dialog")).toHaveCount(0);
});

test("everything is exported with its history, to a file readable by its owner only", async () => {
  await dataTab(seeded);
  await seeded.getByRole("tabpanel").getByRole("button", { name: "Export…" }).click();
  const dialog = seeded.getByRole("dialog", { name: "Export the site manager" });
  await expect(dialog.getByRole("radio", { name: /Everything/ })).toBeChecked();
  await expect(dialog).toContainText("A sensitive file");
  await dialog.getByRole("checkbox", { name: /Include the history/ }).check();
  await dialog.getByRole("button", { name: "Export…" }).click();
  await expect(seeded.getByRole("tabpanel")).toContainText(/Saved to .*vigie-\d{4}-\d{2}-\d{2}\.json\./);

  const name = readdirSync(files).find((f) => /^vigie-\d{4}-\d{2}-\d{2}\.json$/.test(f))!;
  const file = join(files, name);
  expect(statSync(file).mode & 0o777).toBe(0o600);
  const exported = JSON.parse(readFileSync(file, "utf8"));
  expect(exported.format).toBe("vigie");
  expect(exported.history.scans.length).toBeGreaterThan(0);
  copyFileSync(file, join(files, "import.json"));
});

test("the export imported into an empty Vigie gives the tree, the history and the finding set aside", async () => {
  await dataTab(empty);
  await empty.getByRole("tabpanel").getByRole("button", { name: "Import…" }).first().click();
  await expect(empty.getByRole("heading", { level: 1 })).toHaveText("Import import.json");
  await expect(empty.getByRole("main")).toHaveCount(1);
  await expect(empty.getByText(/, with history\. Nothing is added until you confirm\.$/)).toBeVisible();
  await expect(empty.getByRole("heading", { name: /^Added \d+$/ })).toBeVisible();
  await expect(empty.getByRole("heading", { name: /Already in Vigie/ })).toHaveCount(0);
  const add = empty.getByRole("button", { name: /^Add \d+ items$/ });
  const count = Number((await add.textContent())!.match(/\d+/)![0]);
  await add.click();
  await expect(empty.getByRole("tabpanel")).toContainText(`${count} folders, hosts and sites added.`);

  await rail(empty).getByRole("link", { name: "Sites" }).click();
  await tree(empty).getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  await empty.getByRole("tab", { name: "History" }).click();
  await expect(empty.getByRole("list", { name: "History of updates" })).toContainText("woocommerce 9.8.1 → 10.2.0");
  await empty.getByRole("tab", { name: /Security/ }).click();
  await expect(empty.getByRole("tabpanel")).toContainText("1 ignored");
});

test("the same file again adds nothing", async () => {
  await dataTab(empty);
  await empty.getByRole("tabpanel").getByRole("button", { name: "Import…" }).first().click();
  await expect(empty.getByText("Nothing to add: everything is already in Vigie.")).toBeVisible();
  await expect(empty.getByRole("button", { name: "Add 0 items" })).toBeDisabled();
  await empty.getByRole("button", { name: "Cancel" }).click();
  await expect(empty.getByRole("heading", { level: 1 })).toHaveText("Settings");
});

test("a CSV list is previewed with the line refused, then added into a folder", async () => {
  writeFileSync(join(files, "import.csv"), "folder;host;user;path\nCSV;ssh.example.test;atelier;~/www/atelier\nCSV;ssh.example.test;atelier;\n");
  await empty.getByRole("tabpanel").getByRole("button", { name: "Import…" }).last().click();
  await expect(empty.getByText(/^2 lines read, separated by semicolons\./)).toBeVisible();
  await expect(empty.getByText("Line 3: no path")).toBeVisible();
  await empty.getByRole("combobox", { name: "Into" }).selectOption({ label: "Clients" });
  await expect(empty.getByText("Clients/CSV", { exact: true })).toBeVisible();
  await empty.getByRole("button", { name: "Add 3 items" }).click();
  await expect(empty.getByRole("tabpanel")).toContainText("3 folders, hosts and sites added.");
  await rail(empty).getByRole("link", { name: "Sites" }).click();
  await expect(tree(empty).getByRole("treeitem", { name: /^CSV/ })).toBeVisible();
});
