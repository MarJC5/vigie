import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Counts } from "./Counts";

const meta = {
  component: Counts,
  args: {
    label: "Summary",
    items: [
      { label: "Sites", value: 5 },
      { label: "Pending updates", value: 9, tone: "warn" },
      { label: "Need attention", value: 2, tone: "warn" },
      { label: "Not scanned", value: 0 },
    ],
  },
} satisfies Meta<typeof Counts>;
export default meta;

export const HostSummary: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Summary" });
    await expect(list).toHaveTextContent("Pending updates9");
    // A tone is never color alone: the warning shape goes with the number.
    await expect(list!.querySelectorAll("[data-shape='square']")).toHaveLength(2);
  },
};

export const WithAction: StoryObj<typeof meta> = {
  args: { children: <button type="button">Scan host</button> },
  play: async ({ canvas }) => {
    const [action] = canvas.getAllByRole("button", { name: "Scan host" });
    // In the row, outside the list of counts.
    await expect(action!.closest("ul")).toBeNull();
  },
};
