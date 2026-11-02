import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { ItemList } from "./ItemList";

const meta = {
  component: ItemList,
  args: {
    label: "Sites",
    items: [
      { key: "a", icon: "sites", name: "atelier-exemple.ch", note: "1 backup, 38 MB" },
      { key: "b", icon: "sites", name: "boulangerie-exemple.ch" },
      { key: "c", icon: "archive", name: "18/09/2026 09:40", note: "45.1 MB" },
    ],
  },
} satisfies Meta<typeof ItemList>;
export default meta;
type Story = StoryObj<typeof meta>;

// One row per item, its note at the end when it has one.
export const Items: Story = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Sites" });
    await expect(list!.children).toHaveLength(3);
    await expect(list!.children[0]).toHaveTextContent("atelier-exemple.ch1 backup, 38 MB");
  },
};
