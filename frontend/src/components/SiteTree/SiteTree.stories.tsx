import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent } from "storybook/test";

import { sampleTree } from "../../fixtures/tree";
import { SiteTree } from "./SiteTree";

function Controlled() {
  const [selected, setSelected] = useState("atelier");
  return <SiteTree label="Site manager" nodes={sampleTree} selectedId={selected} onSelect={setSelected} />;
}

const meta = {
  component: SiteTree,
  args: {
    label: "Site manager",
    nodes: sampleTree,
    selectedId: "atelier",
    onSelect: () => {},
  },
} satisfies Meta<typeof SiteTree>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree", { name: "Site manager" });
    await expect(tree).toBeInTheDocument();
    // A multisite network shows its subsite count.
    await expect(canvas.getAllByText("14 subsites")).toHaveLength(2);
  },
};

export const Keyboard: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [atelier] = canvas.getAllByRole("treeitem", {
      name: /atelier-exemple.ch/,
    });
    atelier!.focus();
    await userEvent.keyboard("{ArrowDown}");
    const [boulangerie] = canvas.getAllByRole("treeitem", {
      name: /boulangerie-exemple.ch/,
    });
    await expect(boulangerie).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await expect(boulangerie).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowUp}{ArrowUp}");
    const [shared] = canvas.getAllByRole("treeitem", {
      name: /Shared hosting/,
    });
    await expect(shared).toHaveFocus();
  },
};

export const Collapse: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [clients] = canvas.getAllByRole("treeitem", { name: /Clients/ });
    await userEvent.click(clients!.querySelector("[data-caret]")!);
    await expect(clients).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.queryAllByRole("treeitem", { name: /atelier-exemple.ch/ })).toHaveLength(1);
  },
};
