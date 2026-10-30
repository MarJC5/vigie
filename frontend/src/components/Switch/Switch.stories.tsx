import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { Switch } from "./Switch";

const meta = {
  component: Switch,
  args: { label: "WPVulnerability", on: true, onChange: fn() },
} satisfies Meta<typeof Switch>;
export default meta;
type Story = StoryObj<typeof meta>;

// Read as a switch, its state as checked; a click asks for the other state.
export const On: Story = {
  play: async ({ canvas, args }) => {
    const [toggle] = canvas.getAllByRole("switch", { name: "WPVulnerability" });
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await userEvent.click(toggle!);
    await expect(args.onChange).toHaveBeenCalledWith(false);
  },
};

export const Off: Story = {
  args: { on: false },
  play: async ({ canvas, args }) => {
    const [toggle] = canvas.getAllByRole("switch");
    await userEvent.click(toggle!);
    await expect(args.onChange).toHaveBeenCalledWith(true);
  },
};

export const Disabled: Story = {
  args: { disabled: true },
  play: async ({ canvas }) => {
    const [toggle] = canvas.getAllByRole("switch");
    await expect(toggle).toBeDisabled();
  },
};
