import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { InlineField } from "./InlineField";

const meta = {
  component: InlineField,
  args: { label: "Name of ~/sites/network", defaultValue: "Écoles", onCommit: fn(), onCancel: fn() },
} satisfies Meta<typeof InlineField>;
export default meta;
type Story = StoryObj<typeof meta>;

/** Enter confirms with the typed value. */
export const Confirm: Story = {
  play: async ({ canvas, args }) => {
    const [field] = canvas.getAllByRole("textbox", { name: "Name of ~/sites/network" });
    await userEvent.clear(field!);
    await userEvent.type(field!, "Écoles primaires{Enter}");
    await expect(args.onCommit).toHaveBeenCalledWith("Écoles primaires");
  },
};

/** Escape cancels. */
export const Cancel: Story = {
  play: async ({ canvas, args }) => {
    const [field] = canvas.getAllByRole("textbox", { name: "Name of ~/sites/network" });
    await userEvent.type(field!, " bis{Escape}");
    await expect(args.onCancel).toHaveBeenCalled();
    await expect(args.onCommit).not.toHaveBeenCalled();
  },
};

/** 28 px, the field's ground once focused, with the focus ring. */
export const Drawn: Story = {
  args: { autoFocus: true },
  play: async ({ canvas }) => {
    const [field] = canvas.getAllByRole("textbox");
    await expect(field!.getBoundingClientRect().height).toBe(28);
    field!.focus();
    await expect(getComputedStyle(field!).outlineStyle).toBe("solid");
  },
};

export const Invalid: Story = {
  args: { invalid: true, defaultValue: "a,b" },
  play: async ({ canvas }) => {
    const [field] = canvas.getAllByRole("textbox");
    await expect(field).toBeInvalid();
  },
};

/** With a leading icon, as the field that adds a tag. */
export const WithIcon: Story = {
  args: { label: "New tag", defaultValue: "", icon: "tag", placeholder: "Tag" },
};

function Controlled() {
  const [value, setValue] = useState("plain.example.test");
  return (
    <div style={{ width: 280 }}>
      <InlineField label="Name of ~/sites/plain" value={value} onChange={setValue} />
      <output>{value}</output>
    </div>
  );
}

/** Controlled: each key changes the value, as in the list of sites found. */
export const Typed: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [field] = canvas.getAllByRole("textbox", { name: "Name of ~/sites/plain" });
    await userEvent.clear(field!);
    await userEvent.type(field!, "plain");
    await expect(canvas.getAllByText("plain", { selector: "output" })[0]).toBeInTheDocument();
  },
};
