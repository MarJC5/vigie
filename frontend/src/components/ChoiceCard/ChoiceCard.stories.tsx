import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { ChoiceCard } from "./ChoiceCard";

function Pair() {
  const [mode, setMode] = useState<"host" | "alias">("host");
  return (
    <div style={{ display: "grid", gridTemplateColumns: "repeat(2, minmax(0, 1fr))", gap: 14 }}>
      <ChoiceCard
        title="Host name and user"
        text="As your host gives them."
        selected={mode === "host"}
        onSelect={() => setMode("host")}
      />
      <ChoiceCard
        title="Alias from ~/.ssh/config"
        text="An entry you already use in the terminal."
        selected={mode === "alias"}
        onSelect={() => setMode("alias")}
      />
    </div>
  );
}

const meta = {
  component: ChoiceCard,
  args: { title: "Host name and user", text: "As your host gives them.", selected: true, onSelect: fn() },
} satisfies Meta<typeof ChoiceCard>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Selected: Story = {};

export const ChooseOne: Story = {
  render: () => <Pair />,
  play: async ({ canvas }) => {
    const [host] = canvas.getAllByRole("button", { name: /Host name and user/ });
    const [alias] = canvas.getAllByRole("button", { name: /Alias from/ });
    await expect(host).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(alias!);
    await expect(alias).toHaveAttribute("aria-pressed", "true");
    await expect(host).toHaveAttribute("aria-pressed", "false");
  },
};

export const Disabled: Story = {
  args: { title: "A folder or a host", text: "Chosen in the list next.", selected: false, disabled: true },
  play: async ({ canvas, args }) => {
    const [card] = canvas.getAllByRole("button", { name: /A folder or a host/ });
    await expect(card).toBeDisabled();
    await userEvent.click(card!);
    await expect(args.onSelect).not.toHaveBeenCalled();
  },
};
