import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { StepList } from "./StepList";

const meta = {
  component: StepList,
  args: {
    label: "Steps",
    stateLabels: { done: "done", running: "running", failed: "failed", waiting: "next" },
    rows: [
      { key: "a", state: "done", label: "Download", detail: "github.com, 7.1 MB", end: "done" },
      { key: "b", state: "done", label: "Digest check", detail: "SHA-512 of WP-CLI 2.12.0", end: "done" },
      { key: "c", state: "running", label: "Sent over SSH", detail: "4.2 MB of 7.1 MB", end: "running" },
      { key: "d", state: "waiting", label: "Command test", detail: "~/bin/wp cli info", end: "next" },
    ],
  },
} satisfies Meta<typeof StepList>;
export default meta;
type Story = StoryObj<typeof meta>;

// Each step with the icon of its state, named for screen readers; the one under way is the current step.
export const UnderWay: Story = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Steps" });
    await expect(list!.querySelector("[aria-current=step]")).toHaveTextContent("Sent over SSH");
    await expect(canvas.getAllByRole("img", { name: "done" }).length).toBeGreaterThanOrEqual(2);
  },
};

export const Failed: Story = {
  args: {
    ended: true,
    rows: [
      { key: "a", state: "done", label: "Download", detail: "github.com, 7.1 MB", end: "done" },
      { key: "b", state: "failed", label: "Digest check", detail: "SHA-512 of WP-CLI 2.12.0", end: "failed" },
      { key: "c", state: "waiting", label: "Sent over SSH", detail: "to ~/bin/wp", end: "next" },
    ],
  },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "failed" })[0]).toBeInTheDocument();
  },
};
