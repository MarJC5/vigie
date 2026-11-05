import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { Checkbox, CheckboxControl } from "./Checkbox";

const meta = {
  component: Checkbox,
  args: { label: "I compared this fingerprint with the one of my host", checked: false, onChange: fn() },
} satisfies Meta<typeof Checkbox>;
export default meta;
type Story = StoryObj<typeof meta>;

function Toggling() {
  const [checked, setChecked] = useState(false);
  return <Checkbox label="I compared this fingerprint with the one of my host" checked={checked} onChange={setChecked} />;
}

export const Toggle: Story = {
  render: () => <Toggling />,
  play: async ({ canvas }) => {
    const [box] = canvas.getAllByRole("checkbox", { name: "I compared this fingerprint with the one of my host" });
    await expect(box).not.toBeChecked();
    // The whole label is the target, not only the box.
    const [label] = canvas.getAllByText("I compared this fingerprint with the one of my host");
    await userEvent.click(label!);
    await expect(box).toBeChecked();
  },
};

function RowToggling() {
  const [checked, setChecked] = useState(false);
  return (
    <label style={{ display: "grid", gridTemplateColumns: "18px 1fr 1fr", alignItems: "center", gap: 12, cursor: "pointer" }}>
      <CheckboxControl checked={checked} onChange={setChecked} />
      <span>Clients/Valais/Atelier</span>
      <span>ssh.example-host.ch</span>
    </label>
  );
}

/** The drawn box alone, in a row whose own label holds several cells: a click anywhere ticks it. */
export const InARow: Story = {
  render: () => <RowToggling />,
  play: async ({ canvas }) => {
    const [box] = canvas.getAllByRole("checkbox", { name: "Clients/Valais/Atelier ssh.example-host.ch" });
    await expect(box).not.toBeChecked();
    const [cell] = canvas.getAllByText("ssh.example-host.ch");
    await userEvent.click(cell!);
    await expect(box).toBeChecked();
  },
};

export const Keyboard: Story = {
  render: () => <Toggling />,
  play: async ({ canvas }) => {
    const [box] = canvas.getAllByRole("checkbox");
    await userEvent.tab();
    await expect(box).toHaveFocus();
    await userEvent.keyboard(" ");
    await expect(box).toBeChecked();
    await userEvent.keyboard(" ");
    await expect(box).not.toBeChecked();
  },
};

/** Drawn by Vigie: the native box is there for the keyboard and the screen readers, but unseen. */
export const Drawn: Story = {
  args: { checked: true },
  play: async ({ canvas }) => {
    const [box] = canvas.getAllByRole("checkbox");
    await expect(getComputedStyle(box!).opacity).toBe("0");
    const drawn = box!.nextElementSibling as HTMLElement;
    await expect(drawn).toHaveAttribute("aria-hidden", "true");
    await expect(drawn.getBoundingClientRect().width).toBe(18);
    await expect(drawn.getBoundingClientRect().height).toBe(18);
    await expect(getComputedStyle(drawn).borderTopLeftRadius).toBe("5px");
  },
};

export const States: Story = {
  render: () => (
    <div style={{ display: "grid", justifyItems: "start", gap: 14 }}>
      <Checkbox label="Off" checked={false} onChange={fn()} />
      <Checkbox label="On" checked onChange={fn()} />
      <Checkbox label="Off, disabled" checked={false} disabled onChange={fn()} />
      <Checkbox label="On, disabled" checked disabled onChange={fn()} />
      <Checkbox
        label="Include the history"
        note="Scans with their findings, updates and rollbacks: about 3.2 MB more."
        checked
        onChange={fn()}
      />
    </div>
  ),
};

export const Disabled: Story = {
  args: { disabled: true },
  play: async ({ canvas, args }) => {
    const [box] = canvas.getAllByRole("checkbox");
    await expect(box).toBeDisabled();
    const [label] = canvas.getAllByText("I compared this fingerprint with the one of my host");
    await userEvent.click(label!);
    await expect(args.onChange).not.toHaveBeenCalled();
  },
};

/** In a row of a table: the label is read, not shown. */
export const HiddenLabel: Story = {
  args: { label: "Install plain.example.test", hideLabel: true, checked: true },
  play: async ({ canvas }) => {
    const [box] = canvas.getAllByRole("checkbox", { name: "Install plain.example.test" });
    await expect(box).toBeChecked();
    const [text] = canvas.getAllByText("Install plain.example.test");
    await expect(text!.getBoundingClientRect().width).toBeLessThanOrEqual(1);
  },
};
