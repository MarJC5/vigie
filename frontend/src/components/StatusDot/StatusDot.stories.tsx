import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { StatusDot } from "./StatusDot";

const meta = { component: StatusDot, args: { status: "ok" } } satisfies Meta<typeof StatusDot>;
export default meta;
type Story = StoryObj<typeof meta>;

// The meaning is never carried by color alone: each status has its shape and its label ("not fully
// checked" and "unknown" share the ring).
export const AllStatuses: Story = {
  render: () => (
    <div style={{ display: "flex", gap: 20 }}>
      <StatusDot status="ok" withLabel />
      <StatusDot status="warn" withLabel />
      <StatusDot status="danger" withLabel />
      <StatusDot status="partial" withLabel />
      <StatusDot status="unknown" withLabel />
    </div>
  ),
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getAllByText("Needs attention")).toHaveLength(2);
    const shapes = new Set([...canvasElement.querySelectorAll("[data-shape]")].map((e) => e.getAttribute("data-shape")));
    await expect(shapes.size).toBe(4);
  },
};

export const DotAlone: Story = {
  args: { status: "danger" },
  play: async ({ canvas }) => {
    // Without a visible label, the status is still announced.
    await expect(canvas.getAllByRole("img", { name: "Urgent" })).toHaveLength(2);
  },
};

export const OwnLabel: Story = {
  args: { status: "danger", label: "Failed" },
  play: async ({ canvas }) => {
    // A check result says what happened, not a site status.
    await expect(canvas.getAllByRole("img", { name: "Failed" })).toHaveLength(2);
  },
};
