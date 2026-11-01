import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { across } from "../../fixtures/extensions";
import { hostRows } from "../../fixtures/scans";
import { bulk } from "../../fixtures/updates";
import { SitesOverview } from "./SitesOverview";

const meta = {
  component: SitesOverview,
  args: {
    title: "Shared hosting",
    icon: "host",
    subtitle: "u12ab@ssh.example-host.ch · port 22 · wp-cli",
    rows: hostRows,
    scanLabel: "Scan host",
    scanning: false,
    onScan: fn(),
    onSelect: fn(),
  },
} satisfies Meta<typeof SitesOverview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Host: Story = {
  play: async ({ canvas, args }) => {
    const [summary] = canvas.getAllByRole("list", { name: "Summary" });
    await expect(summary).toHaveTextContent("Sites3");
    await expect(summary).toHaveTextContent("Urgent1");
    await expect(summary).toHaveTextContent("Need attention0");
    await expect(summary).toHaveTextContent("Up to date1");
    await expect(summary).toHaveTextContent("Not scanned1");
    // The actions sit on the title line, so the counts keep their row to themselves.
    const scan = canvas.getAllByRole("button", { name: "Scan host" })[0]!;
    await expect(scan.closest("[data-counts]")).toBeNull();
    const [title] = canvas.getAllByRole("heading", { level: 1, name: "Shared hosting" });
    await expect(scan.parentElement!.parentElement).toBe(title!.parentElement);
    await userEvent.click(scan);
    await expect(args.onScan).toHaveBeenCalled();
    await userEvent.click(canvas.getAllByRole("button", { name: "atelier-exemple.ch" })[0]!);
    await expect(args.onSelect).toHaveBeenCalledWith("atelier");
  },
};

export const Folder: Story = {
  args: { title: "Clients", icon: "folder", subtitle: undefined, scanLabel: "Scan now" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("table", { name: "Sites of Clients" })).toHaveLength(2);
  },
};

// A scan of the host running: neither another scan nor an update starts.
export const Scanning: Story = {
  args: { scanning: true, onUpdateAll: fn() },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Scanning…" })[0]).toHaveAttribute("aria-busy", "true");
    await expect(canvas.getAllByRole("button", { name: "Update all" })[0]).toBeDisabled();
    await expect(canvas.queryAllByRole("progressbar")).toHaveLength(0);
  },
};

export const UpdateAll: Story = {
  args: { onUpdateAll: fn() },
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("button", { name: "Update all" })[0]!);
    await expect(args.onUpdateAll).toHaveBeenCalled();
  },
};

// While a site here updates, neither a scan nor another update starts.
export const Updating: Story = {
  args: { onUpdateAll: fn(), updating: true },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Updating…" })[0]).toBeDisabled();
    await expect(canvas.getAllByRole("button", { name: "Scan host" })[0]).toBeDisabled();
  },
};

export const AfterAnUpdate: Story = {
  args: { onUpdateAll: fn(), bulk, onDismissBulk: fn() },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("table", { name: "Result by site" })).toHaveLength(2);
    await expect(canvas.queryAllByRole("table", { name: "Sites of Shared hosting" })).toHaveLength(0);
    await userEvent.click(canvas.getAllByRole("button", { name: "Back to the sites" })[0]!);
    await expect(args.onDismissBulk).toHaveBeenCalled();
  },
};

// The counts filter the table, Show all brings every site back; the headers sort it (ADR-0023).
export const SortAndFilter: Story = {
  play: async ({ canvas }) => {
    const [table] = canvas.getAllByRole("table");
    const names = () => within(table!).getAllByRole("row").slice(1).map((r) => r.querySelector("button")!.textContent);
    const [summary] = canvas.getAllByRole("list", { name: "Summary" });
    const urgent = within(summary!).getByRole("button", { name: /Urgent/ });
    await userEvent.click(urgent);
    await expect(urgent).toHaveAttribute("aria-pressed", "true");
    await expect(names()).toEqual(["boulangerie-exemple.ch"]);
    // A count at zero has nothing to show: not a button.
    await expect(within(summary!).queryByRole("button", { name: /Need attention/ })).toBeNull();
    await userEvent.click(canvas.getAllByRole("button", { name: "Show all" })[0]!);
    await expect(names()).toHaveLength(3);
    // The most updates first, then the other way; the status sorts the worst first.
    const updates = within(table!).getByRole("button", { name: "Updates" });
    await userEvent.click(updates);
    await expect(updates.closest("th")).toHaveAttribute("aria-sort", "descending");
    await expect(names()).toEqual(["boulangerie-exemple.ch", "atelier-exemple.ch", "nouveau.example.test"]);
    await userEvent.click(updates);
    await expect(names()).toEqual(["nouveau.example.test", "atelier-exemple.ch", "boulangerie-exemple.ch"]);
    await userEvent.click(within(table!).getByRole("button", { name: "Status" }));
    await expect(names()[0]).toBe("boulangerie-exemple.ch");
  },
};

// Two tabs on a host: its sites, and their plugins and themes, read when the tab opens (ADR-0024).
export const PluginsTab: Story = {
  args: { extensions: fn(async () => across) },
  play: async ({ canvas, args }) => {
    const [plugins] = canvas.getAllByRole("tab", { name: "Plugins" });
    await expect(args.extensions).not.toHaveBeenCalled();
    await userEvent.click(plugins!);
    await expect(args.extensions).toHaveBeenCalled();
    await expect((await canvas.findAllByRole("table", { name: "Plugins and themes across the sites" })).length).toBeGreaterThan(0);
    const [sites] = canvas.getAllByRole("tab", { name: "Sites" });
    await userEvent.click(sites!);
    await expect(canvas.getAllByRole("list", { name: "Summary" }).length).toBeGreaterThan(0);
  },
};
