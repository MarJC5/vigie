import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Icon, iconNames } from "./Icon";

const meta = { component: Icon, args: { name: "folder" } } satisfies Meta<typeof Icon>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Decorative: Story = {
  play: async ({ canvasElement }) => {
    const svg = canvasElement.querySelector("svg");
    await expect(svg).toHaveAttribute("aria-hidden", "true");
    await expect(svg).toHaveAttribute("width", "16");
  },
};

export const Labelled: Story = {
  args: { name: "warning", label: "Needs attention", size: 20 },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "Needs attention" })).toHaveLength(2);
  },
};

export const Gallery: Story = {
  render: () => (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 12 }}>
      {iconNames.map((name) => (
        <Icon key={name} name={name} size={22} />
      ))}
    </div>
  ),
};
