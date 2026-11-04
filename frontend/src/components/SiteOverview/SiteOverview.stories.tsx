import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { siteChanges } from "../../fixtures/changes";
import { extensions } from "../../fixtures/extensions";
import { debugFatal, fileEditing, findings, phpWeb, xss } from "../../fixtures/findings";
import { networkSite } from "../../fixtures/subsites";
import { failedSite, newSite, offlineSite, outdatedSite, upToDateSite } from "../../fixtures/scans";
import { sampleTags } from "../../fixtures/tree";
import { backups } from "../../fixtures/backups";
import { historyToRollBack } from "../../fixtures/updates";
import { SiteOverview } from "./SiteOverview";

const meta = {
  component: SiteOverview,
  args: { site: outdatedSite, now: new Date("2026-10-29T09:00:00Z"), scanning: false, onScan: fn() },
} satisfies Meta<typeof SiteOverview>;
export default meta;
type Story = StoryObj<typeof meta>;

// Under the title, the quick links (ADR-0023): the site, its dashboard, a shell on its account.
export const QuickLinks: Story = {
  args: { onOpen: fn(), onCopy: fn(async () => {}), terminal: true },
  play: async ({ canvas, args }) => {
    const [links] = canvas.getAllByRole("list", { name: "Site links" });
    const row = within(links!);
    await userEvent.click(row.getByRole("button", { name: "Open site" }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://boulangerie-exemple.ch");
    await userEvent.click(row.getByRole("button", { name: "wp-admin" }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://boulangerie-exemple.ch/wp-admin/");
    await userEvent.click(row.getByRole("button", { name: "SSH terminal" }));
    await expect(args.onOpen).toHaveBeenCalledWith("ssh://u12ab@ssh.example-host.ch:22");
    await userEvent.click(row.getByRole("button", { name: "Copy SSH command" }));
    await expect(args.onCopy).toHaveBeenCalledWith(`ssh -t u12ab@ssh.example-host.ch "cd 'sites/boulangerie' && exec \\$SHELL -l"`);
    await expect(await row.findByRole("button", { name: "Command copied" })).toBeVisible();
  },
};

// Without a handler for ssh:// addresses, no terminal; without an address, no site nor wp-admin.
export const QuickLinksWithoutTerminal: Story = {
  args: { site: newSite, onOpen: fn(), onCopy: fn(), terminal: false },
  play: async ({ canvas }) => {
    const [links] = canvas.getAllByRole("list", { name: "Site links" });
    await expect(within(links!).getAllByRole("button").map((b) => b.textContent)).toEqual(["Copy SSH command"]);
  },
};

// Under the quick links, the site's tags (ADR-0025): one opens its page, one is added.
export const Tags: Story = {
  args: {
    onOpen: fn(), onCopy: fn(async () => {}), terminal: true,
    tags: ["Contrat de maintenance", "Client Dupont"], usedTags: sampleTags, onSaveTags: fn(async () => null), onOpenTag: fn(),
  },
  play: async ({ canvas, args }) => {
    const [tags] = canvas.getAllByRole("list", { name: "Tags" });
    const row = within(tags!);
    await userEvent.click(row.getByRole("button", { name: "Client Dupont" }));
    await expect(args.onOpenTag).toHaveBeenCalledWith("Client Dupont");
    await userEvent.click(row.getByRole("button", { name: "Add a tag" }));
    await userEvent.keyboard("Hébergement{Enter}");
    await expect(args.onSaveTags).toHaveBeenCalledWith(["Contrat de maintenance", "Client Dupont", "Hébergement à migrer"]);
  },
};

export const NeedsAttention: Story = {
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "boulangerie-exemple.ch" })).toHaveLength(2);
    await expect(canvas.getAllByText("Last scan: 3 minutes ago")).toHaveLength(2);
    const [list] = canvas.getAllByRole("tabpanel");
    await expect(list).toHaveTextContent("7.1.3, 7.1.4 available");
    await expect(list).toHaveTextContent("12 installed, 2 to update");
    // The plugins whose updates cannot be seen: a count, the names on demand.
    await expect(list).toHaveTextContent("1 plugin");
    await expect(list).not.toHaveTextContent("Theme Builder Pro");
    // In a dialog, readable however many there are.
    await userEvent.click(within(list!).getByRole("button", { name: "Show" }));
    const [dialog] = canvas.getAllByRole("dialog", { name: "Updates not visible" });
    await expect(dialog).toHaveTextContent("Theme Builder Pro");
    await userEvent.click(within(dialog!).getByRole("button", { name: "Close" }));
    await expect(canvas.queryAllByRole("dialog")).toHaveLength(0);
    // Every row with updates carries its status shape, translations included.
    const translations = within(list!).getByText("Translations").closest("div")!;
    await expect(translations.querySelector("[data-shape='square']")).not.toBeNull();
    const [scan] = canvas.getAllByRole("button", { name: "Scan now" });
    await userEvent.click(scan!);
    await expect(args.onScan).toHaveBeenCalled();
  },
};

// A scan cancelled after a complete one: its results stay, a notice says why nothing newer is.
export const LatestScanCancelled: Story = {
  args: {
    site: {
      ...outdatedSite,
      scan: { ...outdatedSite.scan!, attempt: { finished: "2026-10-29T09:10:00Z", problem: { code: "outcome.cancelled", detail: "cancelled" } } },
    },
  },
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("tabpanel");
    await expect(list).toHaveTextContent("The latest scan did not complete");
    await expect(list).toHaveTextContent("Scan cancelled. These results are from the scan of 29.10.2026");
    await expect(list).toHaveTextContent("7.1.3, 7.1.4 available");
  },
};

// A site being scanned is not updated until the scan ends.
export const UpdatesWhileScanning: Story = {
  args: { onUpdate: fn(), scanning: true },
  play: async ({ canvas }) => {
    const [tab] = canvas.getAllByRole("tab", { name: /Updates/ });
    await userEvent.click(tab!);
    const [update] = canvas.getAllByRole("button", { name: /^Update \(\d+\)$/ });
    await expect(update).toBeDisabled();
  },
};

// The Security tab counts what calls for something; it lists the findings and sets one aside.
export const SecurityTab: Story = {
  args: {
    site: {
      ...outdatedSite,
      scan: { ...outdatedSite.scan!, status: "danger", findings, counts: { urgent: 2, attention: 5, notChecked: 1, ignored: 1 } },
    },
    onIgnore: fn(),
    onCheckFiles: fn(),
  },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const tab = page.getByRole("tab", { name: /Security/ });
    await expect(tab).toHaveTextContent("7");
    await userEvent.click(tab);
    await expect(page.getByRole("tabpanel")).toHaveTextContent("2 urgent · 5 need attention");
    await userEvent.click(page.getByRole("button", { name: /Ignore “Contact Form 7/ }));
    await expect(args.onIgnore).toHaveBeenCalledWith(xss);
  },
};

export const UpdatesTab: Story = {
  args: { onUpdate: fn() },
  play: async ({ canvas, args }) => {
    const [tab] = canvas.getAllByRole("tab", { name: /Updates/ });
    await expect(tab).toHaveTextContent("5");
    await userEvent.click(tab!);
    const [panel] = canvas.getAllByRole("tabpanel");
    await expect(panel).toHaveTextContent("Akismet Anti-spam5.3 → 5.7.2");
    await expect(panel).toHaveTextContent("Not offered: needs a newer PHP or WordPress");
    // Every item offered is chosen; a click leaves one out.
    // Only the first render has its Updates tab open.
    await expect(canvas.getAllByText("4 of 4 selected")).toHaveLength(1);
    await userEvent.click(within(panel!).getByRole("checkbox", { name: /WooCommerce/ }));
    await userEvent.click(canvas.getAllByRole("button", { name: "Update (3)" })[0]!);
    await expect(args.onUpdate).toHaveBeenCalledWith(new Set(["core", "plugin:akismet", "languages"]));
  },
};

export const History: Story = {
  args: { history: historyToRollBack, onRollback: fn(), backups, onRemoveBackups: fn() },
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("tab", { name: "History" })[0]!);
    const [panel] = canvas.getAllByRole("tabpanel");
    await expect(panel).toHaveTextContent("Update of 2 items");
    // The backups on the server first (ADR-0033), then the updates.
    await expect(within(panel!).getByRole("heading", { name: "Backups on the server" })).toBeInTheDocument();
    await userEvent.click(within(panel!).getByRole("button", { name: "Remove" }));
    await expect(args.onRemoveBackups).toHaveBeenCalled();
    await userEvent.click(within(panel!).getByRole("button", { name: "Roll back" }));
    await expect(args.onRollback).toHaveBeenCalledWith("op-failed");
  },
};

// A site in an update cannot be scanned or updated; its window opens again from the page.
export const Updating: Story = {
  args: { updating: true, onUpdate: fn(), onFollow: fn() },
  play: async ({ canvas, args }) => {
    await expect(canvas.queryAllByRole("button", { name: "Scan now" })).toHaveLength(0);
    await userEvent.click(canvas.getAllByRole("button", { name: "Follow the update" })[0]!);
    await expect(args.onFollow).toHaveBeenCalled();
  },
};

export const UpToDate: Story = {
  args: { site: upToDateSite },
  play: async ({ canvas }) => {
    const [tab] = canvas.getAllByRole("tab", { name: /Updates/ });
    await userEvent.click(tab!);
    await expect(canvas.getAllByRole("tabpanel")[0]).toHaveTextContent("Everything is up to date.");
  },
};

export const ChecksDidNotReachWordPressOrg: Story = {
  args: { site: offlineSite },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("tabpanel")[0]).toHaveTextContent("not checked: wordpress.org could not be reached");
  },
};

export const ScanFailed: Story = {
  args: { site: failedSite },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("allow yours in the host's control panel");
    // The raw message, once.
    await expect(alert!.textContent!.split("Operation timed out").length - 1).toBe(1);
    // The notice stands apart from the tabs above it and the rows below it.
    const [panel] = canvas.getAllByRole("tabpanel");
    const tabs = canvas.getAllByRole("tablist")[0]!.getBoundingClientRect();
    const box = alert!.getBoundingClientRect();
    const firstRow = panel!.querySelector("dl > div")!.getBoundingClientRect();
    await expect(box.top - tabs.bottom).toBeGreaterThanOrEqual(16);
    await expect(firstRow.top - box.bottom).toBeGreaterThanOrEqual(16);
  },
};

export const UnreadableAnswer: Story = {
  args: {
    site: {
      ...failedSite,
      scan: {
        ...failedSite.scan!,
        problem: { code: "failure.unreadable", detail: "plugin list: unexpected WP-CLI output: json: cannot unmarshal bool" },
      },
    },
  },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("Vigie could not read the answer of WP-CLI");
    await expect(alert!.textContent!.split("cannot unmarshal bool").length - 1).toBe(1);
  },
};

// The host's WP-CLI command does not exist on the account: the notice says so and opens the host's
// connection test, where Vigie offers to install WP-CLI (ADR-0039).
export const NoWPCLI: Story = {
  args: {
    site: {
      ...failedSite,
      scan: { ...failedSite.scan!, problem: { code: "failure.noWpCli", detail: "WP-CLI command not found: bash: line 1: wp: command not found" } },
    },
    onTestHost: fn(),
  },
  play: async ({ canvas, args }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("WP-CLI was not found on this account");
    await expect(alert).toHaveTextContent("wp: command not found");
    await userEvent.click(canvas.getAllByRole("button", { name: "Test the connection" })[0]!);
    await expect(args.onTestHost).toHaveBeenCalled();
  },
};

export const NeverScanned: Story = {
  args: { site: newSite },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("This site has not been scanned yet.")).toHaveLength(2);
  },
};

// A scan running shows in its button, not as a block of its own above the content.
export const Scanning: Story = {
  args: { scanning: true },
  play: async ({ canvas }) => {
    const [scan] = canvas.getAllByRole("button", { name: "Scanning…" });
    await expect(scan).toHaveAttribute("aria-busy", "true");
    await expect(canvas.queryAllByRole("progressbar")).toHaveLength(0);
  },
};

export const Network: Story = {
  args: { site: { ...newSite, multisite: true } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("tabpanel")[0]).toHaveTextContent("Multisite network");
  },
};

// The site's folder holds no WordPress any more: the notice says so and offers to remove it.
export const SiteNoLongerThere: Story = {
  args: {
    site: {
      ...failedSite,
      scan: {
        ...failedSite.scan!,
        outcome: "cannot-load",
        problem: {
          code: "outcome.notWordPress",
          detail: "WordPress cannot be loaded: no WordPress at this path: Error: This does not seem to be a WordPress installation.",
        },
      },
    },
    onRemove: fn(),
  },
  play: async ({ canvas, args }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("No WordPress at this path any more");
    await expect(alert).toHaveTextContent("This does not seem to be a WordPress installation");
    await userEvent.click(within(alert!).getByRole("button", { name: "Remove from Vigie" }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};

// The Plugins and Themes rows of the Overview open the Plugins tab (ADR-0024).
export const PluginsFromTheOverview: Story = {
  args: { site: { ...outdatedSite, scan: outdatedSite.scan && { ...outdatedSite.scan, extensions } } },
  play: async ({ canvas }) => {
    const [show] = canvas.getAllByRole("button", { name: "Show the plugins" });
    await userEvent.click(show!);
    const [tab] = canvas.getAllByRole("tab", { selected: true });
    await expect(tab).toHaveTextContent("Plugins");
    await expect(canvas.getAllByRole("table", { name: "Plugins" }).length).toBeGreaterThan(0);
  },
};

// Above the overview, what the latest scan found changed since the one before (ADR-0027); absent
// when nothing did.
export const SinceThePreviousScan: Story = {
  args: { site: { ...outdatedSite, changes: siteChanges, changesSince: "2026-10-31T02:00:00Z" } },
  play: async ({ canvasElement }) => {
    const en = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(en.getByRole("heading", { level: 2, name: /^Since the previous scan/ })).toBeInTheDocument();
    const list = within(en.getByRole("list", { name: "Since the previous scan" }));
    await expect(list.getAllByRole("listitem")).toHaveLength(8);
    await expect(list.getByText("Files changed in WordPress: 2 changed, 1 added")).toBeInTheDocument();
    await expect(list.getByText("New vulnerability: WooCommerce 9.8.1, SQL injection without authentication")).toBeInTheDocument();
    await expect(list.getByText("Akismet Anti-spam 5.3 → 5.7.2, by Vigie")).toBeInTheDocument();
  },
};

export const NothingSinceThePreviousScan: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("heading", { name: /^Since the previous scan/ })).toBeNull();
  },
};

// The scan's complements (ADR-0028): PHP on the web as the dashboard noted it, the admin email and
// the one waiting for confirmation, debug.log written by WordPress.
export const Complements: Story = {
  globals: { lang: "fr" },
  args: {
    site: {
      ...outdatedSite,
      scan: {
        ...outdatedSite.scan!,
        phpWeb: { version: "8.2.29", seen: "2026-10-30T09:12:00Z", unknown: "" },
        adminEmail: "contact@boulangerie-exemple.ch",
        adminEmailPending: "marie@boulangerie-exemple.ch",
        debugLog: { path: "wp-content/debug.log", size: 38_200_000, written: true },
      },
    },
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const row = (label: string) => page.getByText(label, { selector: "dt" }).parentElement!;
    await expect(row("PHP sur le web")).toHaveTextContent("8.2.29vu par le tableau de bord le 30.10.2026");
    await expect(row("PHP sur le web")).toHaveTextContent("c'est elle qui sert les visiteurs");
    await expect(row("E-mail d'administration")).toHaveTextContent("contact@boulangerie-exemple.chEn attente de confirmation : marie@boulangerie-exemple.ch");
    await expect(row("debug.log")).toHaveTextContent("38,2 Moécrit par WordPress dans wp-content/debug.log");
  },
};

// Not known: nobody opened the dashboard this week, WordPress writes no debug.log.
export const ComplementsNotKnown: Story = {
  args: { site: { ...outdatedSite, scan: { ...outdatedSite.scan!, adminEmail: "contact@boulangerie-exemple.ch" } } },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    const row = (label: string) => page.getByText(label, { selector: "dt" }).parentElement!;
    await expect(row("PHP on the web")).toHaveTextContent("Not knownThe dashboard was not opened this week");
    await expect(row("debug.log")).toHaveTextContent("NoneWordPress writes none");
  },
};

// A version Vigie cannot name, and a debug.log left from before.
export const ComplementsDiffers: Story = {
  args: {
    site: {
      ...outdatedSite,
      scan: {
        ...outdatedSite.scan!,
        phpWeb: { version: "", seen: "", unknown: "unmatched" },
        adminEmail: "webmaster@boulangerie-exemple.ch",
        debugLog: { path: "wp-content/debug.log", size: 3_100_000, written: false },
      },
    },
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const row = (label: string) => page.getByText(label, { selector: "dt" }).parentElement!;
    await expect(row("PHP on the web")).toHaveTextContent("Differs from the command lineVersion not read");
    await expect(row("debug.log")).toHaveTextContent("3.1 MBleft from before: WordPress no longer writes to it");
  },
};

// A scan made before the complements says nothing of them.
export const ComplementsNotRead: Story = {
  args: { site: { ...outdatedSite, scan: { ...outdatedSite.scan!, phpWeb: { version: "", seen: "", unknown: "" }, adminEmail: "" } } },
  play: async ({ canvas }) => {
    for (const label of ["PHP on the web", "Admin email", "debug.log"]) {
      await expect(canvas.queryAllByText(label, { selector: "dt" })).toHaveLength(0);
    }
  },
};

// In the Security tab: the fatal errors of the last week, the latest's words as they are; the PHP
// checks about the version serving the pages.
export const ComplementsSecurity: Story = {
  globals: { lang: "fr" },
  args: {
    site: {
      ...outdatedSite,
      scan: { ...outdatedSite.scan!, findings: [debugFatal, phpWeb, fileEditing], counts: { urgent: 0, attention: 3, notChecked: 0, ignored: 0 } },
    },
    onIgnore: fn(),
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.click(page.getByRole("tab", { name: /Sécurité/ }));
    const panel = within(page.getByRole("tabpanel"));
    await expect(panel.getByText("3 erreurs fatales dans debug.log ces 7 derniers jours")).toBeInTheDocument();
    await expect(panel.getByText(/Call to undefined function wc_get_order\(\)/, { selector: "code" })).toBeInTheDocument();
    await expect(panel.getByText("PHP 8.2.29 sur le web ne reçoit plus que des correctifs de sécurité")).toBeInTheDocument();
  },
};

// A network's Security tab (ADR-0029): the super administrators first, and the subsites each
// administrator administers.
export const NetworkSecurity: Story = {
  globals: { lang: "fr" },
  args: { site: networkSite, onIgnore: fn() },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.click(page.getByRole("tab", { name: /Sécurité/ }));
    const panel = within(page.getByRole("tabpanel"));
    const titles = panel.getAllByText(/: (super )?administrateur/).map((e) => e.textContent);
    await expect(titles).toEqual([
      "jdupont\u00a0: super administrateur sans double authentification",
      "marie\u00a0: administrateur sans double authentification",
      "admin\u00a0: super administrateur avec double authentification",
    ]);
    await expect(panel.getByText(/marie@ecole-alpha\.ch · depuis le 12\.09\.2024 · administre \/alpha et \/beta/)).toBeInTheDocument();
  },
};
