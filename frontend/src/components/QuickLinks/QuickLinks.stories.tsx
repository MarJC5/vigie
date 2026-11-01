import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { QuickLinks } from "./QuickLinks";

const open = fn();
const copy = fn();

const meta = {
  component: QuickLinks,
  args: {
    label: "Site links",
    links: [
      { label: "Open site", icon: "globe", onClick: open },
      { label: "wp-admin", icon: "external", onClick: open },
      { label: "SSH terminal", icon: "terminal", onClick: open },
      { label: "Copy SSH command", icon: "copy", onClick: copy },
    ],
  },
} satisfies Meta<typeof QuickLinks>;
export default meta;

export const SiteLinks: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Site links" });
    await expect(list!.querySelectorAll("button")).toHaveLength(4);
    const [copyButton] = canvas.getAllByRole("button", { name: "Copy SSH command" });
    await userEvent.click(copyButton!);
    await expect(copy).toHaveBeenCalled();
  },
};
