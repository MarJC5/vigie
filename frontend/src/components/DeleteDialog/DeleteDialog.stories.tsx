import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { sampleTree } from "../../fixtures/tree";
import { DeleteDialog } from "./DeleteDialog";

const node = (id: string) => sampleTree.find((n) => n.id === id)!;

const meta = {
  component: DeleteDialog,
  args: { node: node("shared"), nodes: sampleTree, onDelete: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 460 }}><Story /></div>],
} satisfies Meta<typeof DeleteDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// A host goes with its sites, listed; the focus starts on Cancel, the safe choice.
export const AHost: Story = {
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: 'Delete "Shared hosting"?' }).find((d) => d.contains(document.activeElement))!;
    await expect(dialog).toHaveAccessibleDescription("Its 3 sites, their scans and their update history are removed from Vigie.");
    await expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await expect(within(dialog).getAllByRole("listitem")).toHaveLength(3);
    await expect(within(dialog).getByText("network, 14 subsites")).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete the host" }));
    await expect(args.onDelete).toHaveBeenCalled();
  },
};

export const ASite: Story = {
  args: { node: node("atelier") },
  play: async ({ canvas }) => {
    const [dialog] = canvas.getAllByRole("dialog", { name: 'Delete "atelier-exemple.ch"?' });
    await expect(within(dialog!).getByRole("button", { name: "Delete the site" })).toBeInTheDocument();
  },
};

// A folder that still holds something cannot be deleted: the dialog says why and offers nothing.
export const AFolderNotEmpty: Story = {
  args: { node: node("clients") },
  play: async ({ canvas }) => {
    const [dialog] = canvas.getAllByRole("dialog", { name: 'Delete the folder "Clients"?' });
    await expect(dialog).toHaveAccessibleDescription(/Only an empty folder can be deleted/);
    await expect(within(dialog!).queryByRole("button", { name: "Delete the folder" })).toBeNull();
  },
};
