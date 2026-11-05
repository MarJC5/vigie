import type { Meta, StoryObj } from "@storybook/react-vite";
import { useId, useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { Radio, RadioControl } from "./Radio";

const meta = {
  component: Radio,
  args: { label: "Clients", name: "destination", checked: false, onChange: fn() },
} satisfies Meta<typeof Radio>;
export default meta;
type Story = StoryObj<typeof meta>;

const places = ["Top level", "Clients", "Personal", "Archives"];

function Choosing() {
  const name = useId();
  const [chosen, setChosen] = useState("");
  return (
    <fieldset style={{ display: "grid", justifyItems: "start", gap: 12, border: 0, margin: 0, padding: 0 }}>
      <legend>Destination</legend>
      {places.map((p) => (
        <Radio key={p} label={p} name={name} checked={chosen === p} onChange={() => setChosen(p)} />
      ))}
    </fieldset>
  );
}

export const Choose: Story = {
  render: () => <Choosing />,
  play: async ({ canvas }) => {
    const [clients] = canvas.getAllByRole("radio", { name: "Clients" });
    await expect(clients).not.toBeChecked();
    // The whole label is the target, not only the dot.
    const [label] = canvas.getAllByText("Clients");
    await userEvent.click(label!);
    await expect(clients).toBeChecked();
  },
};

/** The arrow keys of a native radio group: they move the choice within the group. */
export const Keyboard: Story = {
  render: () => <Choosing />,
  play: async ({ canvas }) => {
    const [top] = canvas.getAllByRole("radio", { name: "Top level" });
    const [clients] = canvas.getAllByRole("radio", { name: "Clients" });
    await userEvent.tab();
    await expect(top).toHaveFocus();
    await userEvent.keyboard(" ");
    await expect(top).toBeChecked();
    await userEvent.keyboard("{ArrowDown}");
    await expect(clients).toHaveFocus();
    await expect(clients).toBeChecked();
  },
};

/** Drawn by Vigie: the native radio is there for the keyboard and the screen readers, but unseen. */
export const Drawn: Story = {
  args: { checked: true },
  play: async ({ canvas }) => {
    const [radio] = canvas.getAllByRole("radio");
    await expect(getComputedStyle(radio!).opacity).toBe("0");
    const drawn = radio!.nextElementSibling as HTMLElement;
    await expect(drawn).toHaveAttribute("aria-hidden", "true");
    await expect(drawn.getBoundingClientRect().width).toBe(18);
    await expect(drawn.getBoundingClientRect().height).toBe(18);
    await expect(getComputedStyle(drawn).borderTopLeftRadius).toBe("50%");
  },
};

export const States: Story = {
  render: () => (
    <div style={{ display: "grid", justifyItems: "start", gap: 14 }}>
      <Radio label="Off" name="states-1" checked={false} onChange={fn()} />
      <Radio label="On" name="states-2" checked onChange={fn()} />
      <Radio label="Off, disabled" name="states-3" checked={false} disabled onChange={fn()} />
      <Radio label="On, disabled" name="states-4" checked disabled onChange={fn()} />
    </div>
  ),
};

export const Disabled: Story = {
  args: { disabled: true },
  play: async ({ canvas, args }) => {
    const [radio] = canvas.getAllByRole("radio");
    await expect(radio).toBeDisabled();
    const [label] = canvas.getAllByText("Clients");
    await userEvent.click(label!);
    await expect(args.onChange).not.toHaveBeenCalled();
  },
};

function RowChoosing() {
  const name = useId();
  const [chosen, setChosen] = useState(false);
  return (
    <label style={{ display: "flex", alignItems: "center", gap: 10, cursor: "pointer" }}>
      <RadioControl name={name} checked={chosen} onChange={() => setChosen(true)} />
      <span>Archives</span>
      <span>current place</span>
    </label>
  );
}

/** The drawn dot alone, in a row whose own label holds several cells: a click anywhere chooses it. */
export const InARow: Story = {
  render: () => <RowChoosing />,
  play: async ({ canvas }) => {
    const [radio] = canvas.getAllByRole("radio", { name: "Archives current place" });
    await expect(radio).not.toBeChecked();
    const [cell] = canvas.getAllByText("current place");
    await userEvent.click(cell!);
    await expect(radio).toBeChecked();
  },
};
