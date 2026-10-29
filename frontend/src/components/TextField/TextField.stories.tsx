import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { TextField } from "./TextField";

const meta = {
  component: TextField,
  args: { label: "WP-CLI command", value: "wp", hint: "Infomaniak: wp-cli", onChange: fn() },
} satisfies Meta<typeof TextField>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithHint: Story = {
  play: async ({ canvas }) => {
    const [input] = canvas.getAllByLabelText("WP-CLI command");
    await expect(input).toHaveValue("wp");
    await expect(input).toHaveAccessibleDescription("Infomaniak: wp-cli");
  },
};

function Typing() {
  const [value, setValue] = useState("");
  return <TextField label="Host name" value={value} onChange={setValue} placeholder="ssh.example-host.ch" />;
}

export const Typed: Story = {
  render: () => <Typing />,
  play: async ({ canvas }) => {
    const [input] = canvas.getAllByLabelText("Host name");
    await userEvent.type(input!, "ssh.example.test");
    await expect(input).toHaveValue("ssh.example.test");
  },
};

export const Invalid: Story = {
  args: { label: "Port", value: "70000", error: "The port must be between 1 and 65535." },
  play: async ({ canvas }) => {
    const [input] = canvas.getAllByLabelText("Port");
    await expect(input).toHaveAttribute("aria-invalid", "true");
    await expect(input).toHaveAccessibleDescription("The port must be between 1 and 65535.");
  },
};
