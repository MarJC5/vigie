import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { NavRail } from "./NavRail";

// Each story renders once per theme, so the navigation landmark repeats by construction;
// the end-to-end test checks that the window has one.
const twoThemes = {
  a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } },
};

const meta = {
  component: NavRail,
  args: { current: "sites", onAddHost: fn(), onNavigate: fn() },
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
  play: async ({ canvas, args }) => {
    const [sites] = canvas.getAllByRole("link", { name: "Sites" });
    await expect(sites).toHaveAttribute("aria-current", "page");
    await expect(canvas.getAllByRole("button", { name: "Add a host" })).toHaveLength(2);
    // Settings and Activity open; Help does not exist yet.
    const [settings] = canvas.getAllByRole("link", { name: "Settings" });
    await userEvent.click(settings!);
    await expect(args.onNavigate).toHaveBeenCalledWith("settings");
    const [activity] = canvas.getAllByRole("link", { name: "Activity" });
    await userEvent.click(activity!);
    await expect(args.onNavigate).toHaveBeenCalledWith("activity");
    const [help] = canvas.getAllByRole("link", { name: "Help" });
    await expect(help).toHaveAttribute("aria-disabled", "true");
    await userEvent.click(help!);
    await expect(args.onNavigate).toHaveBeenCalledTimes(2);
  },
};
