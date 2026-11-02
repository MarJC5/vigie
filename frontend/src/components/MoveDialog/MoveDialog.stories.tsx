import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { sampleTree } from "../../fixtures/tree";
import { MoveDialog } from "./MoveDialog";

const withArchives = [...sampleTree, { ...sampleTree[5]!, id: "archives", parentId: "personal", name: "Archives", path: "Personal/Archives" }];

const meta = {
  component: MoveDialog,
  args: { items: [sampleTree.find((n) => n.id === "vps")!], nodes: withArchives, onMove: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 460 }}><Story /></div>],
} satisfies Meta<typeof MoveDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const MoveAHost: Story = {
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: 'Move "Managed VPS"' }).find((d) => d.contains(document.activeElement))!;
    const move = within(dialog).getByRole("button", { name: "Move" });
    await expect(move).toBeDisabled();
    await expect(within(dialog).getByRole("radio", { name: /Personal/ })).toBeDisabled();
    await userEvent.click(within(dialog).getByRole("radio", { name: /Clients/ }));
    await userEvent.click(move);
    await expect(args.onMove).toHaveBeenCalledWith("clients");
  },
};

// A folder is not offered itself nor its own folders.
export const MoveAFolder: Story = {
  args: { items: [sampleTree.find((n) => n.id === "personal")!] },
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog")[0]!;
    await expect(within(dialog).getAllByRole("radio").map((r) => r.closest("label")!.textContent)).toEqual(["Top levelcurrent place", "Clients"]);
    await userEvent.click(within(dialog).getByRole("button", { name: "Cancel" }));
    await expect(args.onClose).toHaveBeenCalled();
  },
};

// Several hosts (ADR-0034): a site selected with them stays under its host, which the dialog says;
// the place where they all are is not offered.
export const MoveSeveralHosts: Story = {
  args: {
    items: [sampleTree.find((n) => n.id === "shared")!, { ...sampleTree.find((n) => n.id === "vps")!, parentId: "clients" }],
    staying: [sampleTree.find((n) => n.id === "boulangerie")!],
  },
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Move 2 hosts" })[0]!;
    await expect(dialog).toHaveAccessibleDescription(
      "Choose the folder that will hold them. Their sites follow them. boulangerie-exemple.ch, selected too, stays under its host.",
    );
    await expect(within(dialog).getByRole("radio", { name: /Clients/ })).toBeDisabled();
    await userEvent.click(within(dialog).getByRole("radio", { name: /Archives/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Move" }));
    await expect(args.onMove).toHaveBeenCalledWith("archives");
  },
};
