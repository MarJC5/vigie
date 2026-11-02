import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { backups } from "../../fixtures/backups";
import { historyToRollBack } from "../../fixtures/updates";
import { BackupList } from "./BackupList";

const meta = {
  component: BackupList,
  args: { backups, operations: historyToRollBack, names: new Map(), onRemove: fn() },
} satisfies Meta<typeof BackupList>;
export default meta;
type Story = StoryObj<typeof meta>;

// Each update's backups with what it was and its size; the one past 30 days marked, removable.
export const PastTheLimit: Story = {
  play: async ({ canvasElement, args }) => {
    const list = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(list.getByText("Read at the last scan · kept 30 days, 3 updates at most")).toBeInTheDocument();
    const rows = list.getAllByRole("listitem");
    await expect(rows).toHaveLength(3);
    await expect(rows[0]).toHaveTextContent(/Update45\.1 MBOver 30 days/);
    await expect(rows[2]).toHaveTextContent(/Update of 2 items48\.2 MB/);
    await expect(list.getByText("1 backup over 30 days old, 45.1 MB")).toBeInTheDocument();
    await userEvent.click(list.getByRole("button", { name: "Remove…" }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};

// While an update or a scan runs on the site, nothing can be removed.
export const Busy: Story = {
  args: { busy: true },
  play: async ({ canvasElement }) => {
    const list = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(list.getByRole("button", { name: "Remove…" })).toBeDisabled();
  },
};

export const NotReadYet: Story = {
  args: { backups: { read: false, days: 30, kept: 3, items: [] } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Not read yet: the next scan reads them.")).toHaveLength(2);
  },
};

export const NoneWithNoLimit: Story = {
  args: { backups: { read: true, days: 0, kept: 3, items: [] } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Read at the last scan · kept with no time limit, 3 updates at most")).toHaveLength(2);
    await expect(canvas.getAllByText("No backups on the server.")).toHaveLength(2);
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText(/^1 sauvegarde de plus de 30 jours, 45,1\sMo$/)).toHaveLength(2);
  },
};
