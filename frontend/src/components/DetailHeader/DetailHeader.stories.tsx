import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { DetailHeader } from "./DetailHeader";

const meta = {
  component: DetailHeader,
  args: { title: "atelier-exemple.ch", meta: "Last scan: 3 minutes ago" },
} satisfies Meta<typeof DetailHeader>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Site: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "atelier-exemple.ch" })).toHaveLength(2);
    await expect(canvas.getAllByText("Last scan: 3 minutes ago")).toHaveLength(2);
  },
};

export const Host: Story = {
  args: {
    title: "Shared hosting",
    icon: "host",
    subtitle: "u12ab@ssh.example-host.ch · port 22 · wp-cli",
  },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("u12ab@ssh.example-host.ch · port 22 · wp-cli")).toHaveLength(2);
  },
};
