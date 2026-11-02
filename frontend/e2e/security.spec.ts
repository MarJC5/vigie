import { expect, type Page, test } from "@playwright/test";

// The security tests set findings aside and answer the consent screen, in order: each browser has
// its own server.
test.describe.configure({ mode: "serial" });

let page: Page;
const tree = () => page.getByRole("tree", { name: "Site manager" });
const panel = () => page.getByRole("tabpanel");

test.beforeAll(async ({ browser }, info) => {
  page = await browser.newPage({ viewport: { width: 1280, height: 800 } });
  await page.goto(info.project.name === "webkit" ? "http://localhost:34120/" : "http://localhost:34119/");
});

test.afterAll(async () => {
  await page.close();
});

test("the Security tab lists the findings by group, the ignored one last", async () => {
  await tree().getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  const tab = page.getByRole("tab", { name: /Security/ });
  await expect(tab).toContainText("2");
  await tab.click();
  await expect(panel()).toContainText("2 need attention · 1 not checked · 1 ignored");
  await expect(panel().getByRole("heading", { level: 2 })).toHaveText(["Needs attention2", "Not checked1", "Information1", "Ignored1"]);
  await expect(panel()).toContainText("Akismet Anti-spam 5.3: Fictional stored cross-site scripting");
  await expect(panel()).toContainText("Ignored permanently on");
  await expect(panel()).toContainText("Vulnerability data from WPVulnerability");
});

test("a finding is set aside with a reason, then put back", async () => {
  await panel().getByRole("button", { name: "Ignore “Files can be edited from the dashboard”" }).click();
  const dialog = page.getByRole("dialog");
  await expect(dialog.getByRole("radio", { name: /Permanently/ })).toBeChecked();
  await expect(dialog.getByRole("button", { name: "Ignore" })).toBeDisabled();
  await dialog.getByRole("textbox", { name: "Reason" }).fill("Edited through deployments only");
  await dialog.getByRole("button", { name: "Ignore" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);
  await expect(panel()).toContainText("1 needs attention · 1 not checked · 2 ignored");
  await expect(panel()).toContainText("“Edited through deployments only”");

  await panel().getByRole("button", { name: "Stop ignoring “Files can be edited from the dashboard”" }).click();
  await expect(panel()).toContainText("2 need attention · 1 not checked · 1 ignored");
});

test("a scan before the user answered asks first, and the settings agree or withdraw later", async () => {
  await tree().getByRole("treeitem", { name: /atelier-exemple\.ch/ }).click();
  await page.getByRole("button", { name: "Scan now" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("Before the first security scan");
  await expect(page.getByRole("main")).toHaveCount(1);
  await expect(page.getByRole("switch")).toHaveCount(5);
  await page.getByRole("button", { name: "Not now" }).click();
  await expect(tree()).toBeVisible();

  // Asked once: the next scan does not ask again; the settings say nothing was agreed.
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Settings" }).click();
  await page.getByRole("tab", { name: "Lookups outside" }).click();
  await expect(panel()).toContainText("Not given: nothing is sent");
  await panel().getByRole("button", { name: "Agree" }).click();
  await expect(panel()).toContainText("Given on");
  await panel().getByRole("button", { name: "Withdraw consent" }).click();
  await expect(panel()).toContainText("Not given: nothing is sent");
});

// An update ignored (ADR-0026): apart in the Updates tab, counted nowhere, said on the Plugins tab,
// then put back.
test("an update is ignored with a reason, then put back", async () => {
  await page.getByRole("navigation", { name: "Navigation" }).getByRole("link", { name: "Sites" }).click();
  await tree().getByRole("treeitem", { name: /boulangerie-exemple\.ch/ }).click();
  const tab = page.getByRole("tab", { name: /^Updates/ });
  await expect(tab).toContainText("3");
  await tab.click();
  await expect(panel().getByRole("link", { name: /Changelog/ })).toBeVisible();

  await panel().getByRole("button", { name: "Ignore the update of Akismet Anti-spam" }).click();
  const dialog = page.getByRole("dialog", { name: "Ignore the update of Akismet Anti-spam?" });
  await dialog.getByRole("radio", { name: /Permanently/ }).check();
  await dialog.getByRole("textbox").fill("The theme uses its internal API");
  await dialog.getByRole("button", { name: "Ignore" }).click();
  await expect(page.getByRole("dialog")).toHaveCount(0);

  const ignored = panel().getByRole("group", { name: /^Ignored/ });
  await expect(ignored).toContainText("“The theme uses its internal API”");
  await expect(ignored.getByRole("checkbox", { name: /Akismet/ })).not.toBeChecked();
  await expect(tab).toContainText("2");
  await page.getByRole("tab", { name: "Plugins" }).click();
  await expect(panel()).toContainText("→ 5.7.2 · ignored");

  await tab.click();
  await panel().getByRole("button", { name: "Stop ignoring the update of Akismet Anti-spam" }).click();
  await expect(panel().getByRole("group", { name: /^Ignored/ })).toHaveCount(0);
  await expect(tab).toContainText("3");
});
