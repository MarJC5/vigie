import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { TreeNode } from "./TreeNode";

const meta = {
  component: TreeNode,
  args: { kind: "site", name: "atelier-exemple.ch", depth: 2, onSelect: fn(), onToggle: fn() },
  decorators: [(Story) => <div role="tree" aria-label="Site manager"><Story /></div>],
} satisfies Meta<typeof TreeNode>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Tree: Story = {
  render: (args) => (
    <>
      <TreeNode kind="folder" name="Clients" depth={0} expanded color="blue" onSelect={args.onSelect} onToggle={args.onToggle} />
      <TreeNode kind="host" name="Shared hosting" depth={1} expanded onSelect={args.onSelect} onToggle={args.onToggle} />
      <TreeNode kind="site" name="atelier-exemple.ch" depth={2} status="ok" selected onSelect={args.onSelect} />
      <TreeNode kind="site" name="boulangerie-exemple.ch" depth={2} status="warn" color="orange" onSelect={args.onSelect} />
      <TreeNode kind="site" name="reseau-ecoles.ch" depth={2} status="ok" meta="14 subsites" onSelect={args.onSelect} />
      <TreeNode kind="folder" name="Personal" depth={0} expanded={false} meta="3" onSelect={args.onSelect} onToggle={args.onToggle} />
    </>
  ),
};

export const Selection: Story = {
  args: { status: "warn" },
  play: async ({ canvas, args }) => {
    const [item] = canvas.getAllByRole("treeitem", { name: /atelier-exemple.ch/ });
    await expect(item).toHaveAttribute("aria-level", "3");
    await userEvent.click(item!);
    await expect(args.onSelect).toHaveBeenCalled();
  },
};

export const Folder: Story = {
  args: { kind: "folder", name: "Clients", depth: 0, expanded: true },
  play: async ({ canvas, args }) => {
    const [item] = canvas.getAllByRole("treeitem", { name: /Clients/ });
    await expect(item).toHaveAttribute("aria-expanded", "true");
    item!.focus();
    await userEvent.keyboard("{ArrowLeft}");
    await expect(args.onToggle).toHaveBeenCalled();
  },
};
