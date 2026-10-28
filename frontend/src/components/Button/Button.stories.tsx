import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { Button } from "./Button";

const meta = {
  component: Button,
  args: { children: "Scan now", onClick: fn() },
} satisfies Meta<typeof Button>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Primary: Story = {
  args: { variant: "primary", icon: "refresh" },
  play: async ({ canvas, args }) => {
    const [button] = canvas.getAllByRole("button", { name: "Scan now" });
    await userEvent.click(button!);
    await expect(args.onClick).toHaveBeenCalledOnce();
  },
};

export const Outline: Story = { args: { variant: "outline", children: "wp-admin", trailingIcon: "external" } };

export const Text: Story = { args: { variant: "text", children: "Details", trailingIcon: "external" } };

export const IconOnly: Story = {
  args: { variant: "icon", icon: "search", label: "Search", children: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Search" })).toHaveLength(2);
  },
};

export const Disabled: Story = {
  args: { variant: "primary", disabled: true },
  play: async ({ canvas, args }) => {
    const [button] = canvas.getAllByRole("button", { name: "Scan now" });
    await expect(button).toBeDisabled();
    await userEvent.click(button!, { pointerEventsCheck: 0 });
    await expect(args.onClick).not.toHaveBeenCalled();
  },
};
