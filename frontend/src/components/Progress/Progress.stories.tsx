import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Progress } from "./Progress";

const meta = {
  component: Progress,
  args: { label: "Searching the account for WordPress installs…" },
} satisfies Meta<typeof Progress>;
export default meta;
type Story = StoryObj<typeof meta>;

// The length of the wait is unknown: the bar moves, the text says what happens.
export const Unknown: Story = {
  play: async ({ canvas }) => {
    const [bar] = canvas.getAllByRole("progressbar", { name: "Searching the account for WordPress installs…" });
    await expect(bar).not.toHaveAttribute("aria-valuenow");
  },
};

export const Counted: Story = {
  args: { label: "Checking the installs found: 3 of 6", value: 3, max: 6 },
  play: async ({ canvas }) => {
    const [bar] = canvas.getAllByRole("progressbar", { name: "Checking the installs found: 3 of 6" });
    await expect(bar).toHaveAttribute("aria-valuenow", "3");
    await expect(bar).toHaveAttribute("aria-valuemax", "6");
    await expect(canvas.getAllByText("Checking the installs found: 3 of 6")).toHaveLength(2);
  },
};

export const NothingCheckedYet: Story = {
  args: { label: "Checking the installs found: 0 of 6", value: 0, max: 6 },
  play: async ({ canvas }) => {
    const [bar] = canvas.getAllByRole("progressbar", { name: "Checking the installs found: 0 of 6" });
    // An empty bar at zero, not a segment.
    await expect(bar!.firstElementChild!.getBoundingClientRect().width).toBe(0);
  },
};
