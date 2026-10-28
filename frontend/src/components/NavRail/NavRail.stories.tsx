import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn } from "storybook/test";

import { NavRail } from "./NavRail";

// Each story renders once per theme, so the navigation landmark repeats by construction;
// the end-to-end test checks that the window has one.
const twoThemes = {
  a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } },
};

const meta = {
  component: NavRail,
  args: { current: "sites", onAddHost: fn() },
  parameters: twoThemes,
} satisfies Meta<typeof NavRail>;
export default meta;

export const Default: StoryObj<typeof meta> = {
  decorators: [
    (Story) => (
      <div style={{ height: 420, display: "flex" }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ canvas }) => {
    const [sites] = canvas.getAllByRole("link", { name: "Sites" });
    await expect(sites).toHaveAttribute("aria-current", "page");
    await expect(canvas.getAllByRole("button", { name: "Add a host" })).toHaveLength(2);
  },
};
