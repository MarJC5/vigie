import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { AddonTrustDialog } from "./AddonTrustDialog";

const meta = {
  component: AddonTrustDialog,
  args: { onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 460 }}><Story /></div>],
} satisfies Meta<typeof AddonTrustDialog>;
export default meta;

export const WhatAnAddOnCanDo: StoryObj<typeof meta> = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getByRole("heading", { name: "A program of its own" })).toBeInTheDocument();
    await expect(dialog.getByText(/stops it after two minutes/)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Understood" }));
    await expect(args.onClose).toHaveBeenCalled();
  },
};
