import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { TextArea } from "./TextArea";

const meta = {
  component: TextArea,
  args: { label: "Reason", value: "", onChange: fn() },
} satisfies Meta<typeof TextArea>;
export default meta;
type Story = StoryObj<typeof meta>;

function Typing() {
  const [value, setValue] = useState("");
  return <TextArea label="Reason" hint="Kept with the ignore, to know later why." value={value} onChange={setValue} />;
}

export const Typed: Story = {
  render: () => <Typing />,
  play: async ({ canvas }) => {
    const [area] = canvas.getAllByRole("textbox", { name: "Reason" });
    await userEvent.type(area!, "Online payment not tested with WooCommerce 10");
    await expect(area).toHaveValue("Online payment not tested with WooCommerce 10");
    // The hint is read with the field.
    await expect(area).toHaveAccessibleDescription("Kept with the ignore, to know later why.");
  },
};

/** TextField's look on several lines: the field's ground, the same corners and padding. */
export const SameAsTextField: Story = {
  args: { value: "Online payment not tested with WooCommerce 10" },
  play: async ({ canvas }) => {
    const [area] = canvas.getAllByRole("textbox", { name: "Reason" });
    const style = getComputedStyle(area!);
    await expect(style.borderTopLeftRadius).toBe("8px");
    await expect(style.resize).toBe("none");
  },
};

export const Invalid: Story = {
  args: { error: "A reason is needed to ignore permanently.", required: true },
  play: async ({ canvas }) => {
    const [area] = canvas.getAllByRole("textbox", { name: "Reason" });
    await expect(area).toBeInvalid();
    await expect(area).toBeRequired();
    await expect(area).toHaveAccessibleDescription("A reason is needed to ignore permanently.");
  },
};

/** A refusal after sending: the error is announced. */
export const Refused: Story = {
  args: { error: "The reason could not be saved.", alert: true },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("The reason could not be saved.");
  },
};

/** The notes of a site: monospace, tall, resized by the user; the label read, not shown. */
export const Notes: Story = {
  args: {
    label: "Notes on boulangerie-exemple.ch",
    hideLabel: true,
    variant: "notes",
    value: "## Access\n\nThe client's FTP is closed since March.\n\n- [ ] Renew the certificate",
  },
  play: async ({ canvas }) => {
    const [area] = canvas.getAllByRole("textbox", { name: "Notes on boulangerie-exemple.ch" });
    const style = getComputedStyle(area!);
    await expect(style.resize).toBe("vertical");
    await expect(style.fontFamily).toMatch(/mono|Menlo/i);
    await expect(canvas.queryAllByText("Notes on boulangerie-exemple.ch")[0]!.getBoundingClientRect().width).toBeLessThanOrEqual(1);
  },
};
