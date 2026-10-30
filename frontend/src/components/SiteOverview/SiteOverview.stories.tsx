import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { failedSite, newSite, offlineSite, outdatedSite, upToDateSite } from "../../fixtures/scans";
import { historyToRollBack } from "../../fixtures/updates";
import { SiteOverview } from "./SiteOverview";

const meta = {
  component: SiteOverview,
  args: { site: outdatedSite, now: new Date("2026-10-29T09:00:00Z"), scanning: false, onScan: fn() },
} satisfies Meta<typeof SiteOverview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const NeedsAttention: Story = {
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "boulangerie-exemple.ch" })).toHaveLength(2);
    await expect(canvas.getAllByText("Last scan: 3 minutes ago")).toHaveLength(2);
    const [list] = canvas.getAllByRole("tabpanel");
    await expect(list).toHaveTextContent("7.1.3 · 7.1.4 available");
    await expect(list).toHaveTextContent("12 · 2 updates");
    // The plugins whose updates cannot be seen: a count, the names on demand.
    await expect(list).toHaveTextContent("1 plugin");
    await expect(list).not.toHaveTextContent("Theme Builder Pro");
    await userEvent.click(within(list!).getByRole("button", { name: "Show" }));
    await expect(list).toHaveTextContent("Theme Builder Pro");
    await expect(within(list!).getByRole("button", { name: "Hide" })).toHaveAttribute("aria-expanded", "true");
    // Every row with updates carries its status shape, translations included.
    const translations = within(list!).getByText("Translations").closest("div")!;
    await expect(translations.querySelector("[data-shape='square']")).not.toBeNull();
    const [scan] = canvas.getAllByRole("button", { name: "Scan now" });
    await userEvent.click(scan!);
    await expect(args.onScan).toHaveBeenCalled();
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
  args: { history: historyToRollBack, onRollback: fn() },
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("tab", { name: "History" })[0]!);
    const [panel] = canvas.getAllByRole("tabpanel");
    await expect(panel).toHaveTextContent("Update of 2 items");
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
    await userEvent.click(within(alert!).getByRole("button", { name: "Remove from Vigie…" }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};
