import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { listSource } from "../../fixtures/addons";
import { AddonImage } from "./AddonImage";

const meta = { component: AddonImage, args: { image: listSource.image } } satisfies Meta<typeof AddonImage>;
export default meta;
type Story = StoryObj<typeof meta>;

// The add-on's own image, shown as an image: an SVG runs no script there.
export const ItsOwn: Story = {
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelectorAll("img")).toHaveLength(2);
  },
};

export const TheDefaultOne: Story = {
  args: { image: "", size: "hero" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("No image of its own")).toHaveLength(2);
  },
};
