import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { Checkbox } from "./Checkbox";

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
