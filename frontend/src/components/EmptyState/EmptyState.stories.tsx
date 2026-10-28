import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Button } from "../Button/Button";
import { EmptyState } from "./EmptyState";

const meta = {
  component: EmptyState,
  args: {
    title: "Add your first host",
    text: "Vigie connects to your servers over SSH with the keys you already use, and runs WP-CLI there. Nothing is installed on your sites.",
    children: (
      <>
        <Button variant="primary" icon="plus">Add a host</Button>
        <Button variant="text">Import from FileZilla</Button>
      </>
    ),
  },
} satisfies Meta<typeof EmptyState>;
export default meta;

export const FirstLaunch: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Add your first host" })).toHaveLength(2);
  },
};
