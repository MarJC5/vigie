import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { outdatedSite } from "../../fixtures/scans";
import { historyAuto, historyRolledBack, historyToRollBack } from "../../fixtures/updates";
import { namesOf } from "../../lib/updates";
import { UpdateHistory } from "./UpdateHistory";

const meta = {
  component: UpdateHistory,
  args: { operations: historyToRollBack, names: namesOf(outdatedSite.scan!.updates!), onRollback: fn() },
} satisfies Meta<typeof UpdateHistory>;
export default meta;
type Story = StoryObj<typeof meta>;

export const ToRollBack: Story = {
  play: async ({ canvas, args }) => {
    const [list] = canvas.getAllByRole("list", { name: "History of updates" });
    // The newest is open, with its steps and the rollback of its failed item.
    const [newest] = canvas.getAllByRole("button", { expanded: true });
    const [older] = canvas.getAllByRole("button", { expanded: false });
    await expect(newest).toHaveTextContent("Update of 2 items");
    await expect(newest).toHaveTextContent("Failed");
    await expect(list).toHaveTextContent("wc_get_page_id");
    await expect(list).toHaveTextContent("Backup kept on the server:");
    await userEvent.click(canvas.getAllByRole("button", { name: "Roll back" })[0]!);
    await expect(args.onRollback).toHaveBeenCalledWith("op-failed");
    // An older one opens on a click.
    await userEvent.click(older!);
    await expect(older).toHaveAttribute("aria-expanded", "true");
    await expect(older).toHaveTextContent("1 min 31 s");
  },
};

export const RolledBack: Story = {
  args: { operations: historyRolledBack },
  play: async ({ canvas }) => {
    const [newest] = canvas.getAllByRole("button", { expanded: true });
    await expect(newest).toHaveTextContent("WooCommerce back to 9.8.1");
    await expect(canvas.queryAllByRole("button", { name: "Roll back" })).toHaveLength(0);
  },
};

export const Busy: Story = {
  args: { busy: true },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Roll back" })[0]).toBeDisabled();
  },
};

export const Empty: Story = {
  args: { operations: [] },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("No update yet.")).toHaveLength(2);
  },
};

// ADR-0042: the rollback Vigie ran on its own, above the update it came from, which is open.
export const RolledBackOnItsOwn: Story = {
  args: { operations: historyAuto },
  play: async ({ canvas }) => {
    const [first] = canvas.getAllByRole("button", { expanded: false });
    await expect(first).toHaveTextContent("WooCommerce back to 9.8.1, on its own");
    const [update] = canvas.getAllByRole("button", { expanded: true });
    await expect(update).toHaveTextContent("Done, 1 item rolled back");
    const [list] = canvas.getAllByRole("list", { name: "History of updates" });
    await expect(list).toHaveTextContent("an operation of its own, above");
    await expect(canvas.queryAllByRole("button", { name: "Roll back" })).toHaveLength(0);
  },
};
