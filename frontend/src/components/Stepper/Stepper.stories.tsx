import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Stepper } from "./Stepper";

const meta = {
  component: Stepper,
  args: { label: "Steps", steps: ["Connection", "Server key", "Test", "Sites"], current: 1, doneLabel: "done" },
} satisfies Meta<typeof Stepper>;
export default meta;

export const SecondStep: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Steps" });
    const items = list!.querySelectorAll("li");
    await expect(items).toHaveLength(4);
    await expect(items[1]).toHaveAttribute("aria-current", "step");
    await expect(items[0]).not.toHaveAttribute("aria-current");
    // A finished step says so to screen readers, not only with its check mark.
    await expect(items[0]).toHaveTextContent("done");
  },
};
