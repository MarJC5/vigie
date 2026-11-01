import { expect, test } from "@playwright/test";

// A network's subsites (ADR-0029), on the seed of a network scanned (seed -network), in French:
// unfolded in the site list, each with its page; where each plugin is active; the super
// administrators first. The tests only read.
const network = "http://localhost:34129/";

test("a network unfolds to its subsites, each with its page", async ({ page }) => {
  await page.goto(network);
  const tree = page.getByRole("tree", { name: "Gestionnaire de sites" });
  const site = tree.getByRole("treeitem", { name: /reseau-ecoles\.ch/ });
  // Folded by default (the story checks it); both browsers share this server, which keeps the
  // list as the other left it.
  if ((await site.getAttribute("aria-expanded")) === "false") await site.locator("[data-caret]").click();
  await expect(site).toHaveAttribute("aria-expanded", "true");
  await expect(tree.getByRole("treeitem", { name: /^\/gamma/ })).toContainText("archivé");

  await tree.getByRole("treeitem", { name: "/alpha" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("reseau-ecoles.ch/alpha");
  const row = (label: string) => page.locator("dt", { hasText: new RegExp(`^${label}$`) }).locator("..");
  await expect(row("Thème")).toContainText("Astra 4.8.1");
  await expect(row("Extensions actives")).toContainText("1 sur ce sous-site");
  await expect(row("Extensions actives")).toContainText("+ 1 sur tout le réseau");
  await expect(page.getByRole("list", { name: "Administrateurs" })).toContainText("marie : administrateur sans double authentification");

  // Archived: its state, no details.
  await tree.getByRole("treeitem", { name: /^\/gamma/ }).click();
  await expect(page.getByRole("note")).toContainText("Ce sous-site est archivé");

  // Back to the network.
  await page.getByRole("button", { name: "reseau-ecoles.ch", exact: true }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("reseau-ecoles.ch");
});

test("a network's Plugins tab says where each is active, its Security tab the super administrators first", async ({ page }) => {
  await page.goto(network);
  await page.getByRole("tree", { name: "Gestionnaire de sites" }).getByRole("treeitem", { name: /reseau-ecoles\.ch/ }).click();
  await page.getByRole("tab", { name: /Extensions/ }).click();
  const plugins = page.getByRole("table", { name: "Extensions" });
  await expect(plugins.getByRole("columnheader", { name: "Activation" })).toBeVisible();
  await expect(plugins.getByRole("row", { name: /WooCommerce/ })).toContainText("Sur tout le réseau");
  await expect(plugins.getByRole("row", { name: /Hello Dolly/ })).toContainText("Nulle part");
  await plugins.getByRole("button", { name: "Sur 2 sous-sites" }).click();
  await page.getByRole("list", { name: "Sous-sites où Contact Form 7 est actif" }).getByRole("button", { name: "/beta" }).click();
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("reseau-ecoles.ch/beta");
  // The network unfolded in the list, the subsite chosen.
  await expect(page.getByRole("treeitem", { name: "/beta" })).toHaveAttribute("aria-selected", "true");

  await page.getByRole("button", { name: "reseau-ecoles.ch", exact: true }).click();
  await page.getByRole("tab", { name: /Sécurité/ }).click();
  const titles = page.getByRole("tabpanel").getByText(/\u00a0: (super )?administrateur/);
  await expect(titles).toHaveText([
    "jdupont : super administrateur sans double authentification",
    "marie : administrateur sans double authentification",
    "admin : super administrateur avec double authentification",
  ]);
  await expect(page.getByRole("tabpanel")).toContainText("administre /alpha et /beta");
});
