import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { FieldNote } from "./FieldNote";

const meta = {
  component: FieldNote,
  args: { children: "The name shown in Vigie." },
} satisfies Meta<typeof FieldNote>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Hint: Story = {
  play: async ({ canvas }) => {
    const [note] = canvas.getAllByText("The name shown in Vigie.");
    await expect(note).not.toHaveAttribute("role");
    await expect(getComputedStyle(note!).fontSize).toBe("12px");
  },
};

/** An error: in the text's color, the field around it carries the red edge. */
export const Error: Story = {
  args: { children: "Between 1 and 65535.", error: true },
};

/** A refusal after sending: announced as it comes. */
export const Alert: Story = {
  args: { children: "A reason is needed to ignore permanently.", error: true, announce: "alert" },
  play: async ({ canvas }) => {
    const [note] = canvas.getAllByRole("alert");
    await expect(note).toHaveTextContent("A reason is needed to ignore permanently.");
  },
};

/** A hint that changes as one types: read politely. */
export const Polite: Story = {
  args: { children: "Enter adds the tag, Escape cancels.", announce: "polite" },
  play: async ({ canvas }) => {
    const [note] = canvas.getAllByText("Enter adds the tag, Escape cancels.");
    await expect(note).toHaveAttribute("aria-live", "polite");
  },
};
