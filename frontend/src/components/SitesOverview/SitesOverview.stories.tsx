import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { hostRows } from "../../fixtures/scans";
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
    await expect(summary).toHaveTextContent("Pending updates4");
    await expect(summary).toHaveTextContent("Need attention1");
    await expect(summary).toHaveTextContent("Not scanned1");
    // The action sits at the right of the counts, as on the validated screen.
    const scan = canvas.getAllByRole("button", { name: "Scan host" })[0]!;
    await expect(scan.closest("[data-counts]")).not.toBeNull();
    await expect(scan.closest("[data-counts]")).toBe(summary!.closest("[data-counts]"));
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
