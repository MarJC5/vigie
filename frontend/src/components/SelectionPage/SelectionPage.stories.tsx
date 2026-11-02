import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import type { SiteRow } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { exploited } from "../../fixtures/findings";
import { bulk } from "../../fixtures/updates";
import { SelectionPage } from "./SelectionPage";

const row = (name: string, path: string, status: string, pending: number, worst: SiteRow["worst"] = null): SiteRow => ({
  id: name, name, path: `${path}/${name}`, status, wordpress: "7.1.4", phpCli: "8.4.27", pending, worst,
});

// Two hosts and a site selected, as in the mockups: six sites from three places (short names: each
// theme has half the width of the window).
const rows = [
  row("cabinet.ch", "Clients/Infomaniak", "danger", 4, exploited),
  row("boulangerie.ch", "Clients/Shared", "warn", 5),
  row("vieux-site.ch", "Personal/VPS", "warn", 2),
  row("atelier.ch", "Clients/Shared", "ok", 0),
  row("ecoles.ch", "Clients/Shared", "ok", 0),
  row("menuiserie.ch", "Clients/Infomaniak", "ok", 0),
];

const meta = {
  component: SelectionPage,
  args: {
    title: "3 items selected",
    subtitle: "2 hosts, 1 site · 6 sites in all",
    rows,
    scanLabel: "Scan the 6 sites",
    scanning: false,
    onScan: fn(),
    onUpdateAll: fn(),
    onMove: fn(),
    onColor: fn(),
    onTags: fn(),
    onDelete: fn(),
    onSelect: fn(),
  },
} satisfies Meta<typeof SelectionPage>;
export default meta;
type Story = StoryObj<typeof meta>;

// The actions on all of it, the counts over its sites, the table that says where each is.
export const ThreeItems: Story = {
  play: async ({ canvas, args }) => {
    const [summary] = canvas.getAllByRole("list", { name: "Summary" });
    await expect(summary).toHaveTextContent("Sites6");
    await expect(summary).toHaveTextContent("Urgent1");
    await expect(canvas.getAllByRole("table", { name: "Sites of the selection" })[0]).toHaveTextContent("Personal / VPS");
    await userEvent.click(canvas.getAllByRole("button", { name: "Scan the 6 sites" })[0]!);
    await expect(args.onScan).toHaveBeenCalled();
    await userEvent.click(canvas.getAllByRole("button", { name: "Color" })[0]!);
    await expect(args.onColor).toHaveBeenCalled();
  },
};

// Only sites selected: nothing to move.
export const OnlySites: Story = {
  render: (args) => <SelectionPage {...args} onMove={undefined} />,
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("button", { name: "Move to…" })).toBeNull();
  },
};

// How the update of all of them ended, until dismissed.
export const AfterAnUpdate: Story = {
  args: { bulk, onDismissBulk: fn() },
};
