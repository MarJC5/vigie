import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { CommandLine } from "./CommandLine";

const command = "rm -rf ~/.vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10";

const meta = {
  component: CommandLine,
  args: { command, onCopy: fn() },
} satisfies Meta<typeof CommandLine>;
export default meta;
type Story = StoryObj<typeof meta>;

// The command, selectable; the button copies it and says so.
export const Copied: Story = {
  play: async ({ canvasElement, args }) => {
    const box = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(box.getByText(command)).toBeInTheDocument();
    await userEvent.click(box.getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith(command);
    await expect(box.getByRole("button", { name: "Copied" })).toBeInTheDocument();
  },
};

// Without a clipboard, only the command.
export const WithoutCopy: Story = {
  args: { onCopy: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.queryAllByRole("button")).toHaveLength(0);
  },
};
