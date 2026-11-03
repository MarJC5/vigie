import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { CheckList } from "./CheckList";

const stateLabels = { ok: "Passed", warn: "To check", failed: "Failed", waiting: "Not run yet", running: "In progress" };

const meta = {
  component: CheckList,
  args: {
    label: "Checks",
    stateLabels,
    items: [
      { label: "SSH connection", value: "Connected as u12ab", state: "ok" },
      { label: "PHP on the command line", value: "8.4.27", state: "ok" },
      { label: "WP-CLI", value: "2.12.0 (wp)", state: "ok" },
    ],
  },
} satisfies Meta<typeof CheckList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Passed: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "Passed" })).toHaveLength(6);
  },
};

export const FailedFirst: Story = {
  args: {
    items: [
      { label: "SSH connection", value: "No answer from the server", state: "failed" },
      { label: "PHP on the command line", value: "Not run yet", state: "waiting" },
      { label: "WP-CLI", value: "Not run yet", state: "waiting" },
    ],
  },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "Failed" })).toHaveLength(2);
    await expect(canvas.getAllByRole("img", { name: "Not run yet" })).toHaveLength(4);
  },
};

export const Running: Story = {
  args: { items: [{ label: "SSH connection", value: "", state: "running" }] },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "In progress" })).toHaveLength(2);
  },
};
