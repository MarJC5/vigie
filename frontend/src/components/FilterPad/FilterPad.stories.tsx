import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { FilterPad } from "./FilterPad";

const meta = {
  component: FilterPad,
  args: { label: "Active", count: 4, pressed: false, onSelect: fn() },
} satisfies Meta<typeof FilterPad>;
export default meta;
type Story = StoryObj<typeof meta>;

const filters = [
  ["All", 6],
  ["Active", 4],
  ["Inactive", 2],
  ["With an update", 2],
] as const;

function Row() {
  const [chosen, setChosen] = useState("All");
  return (
    <div role="group" aria-label="Show" style={{ display: "flex", gap: 4 }}>
      {filters.map(([label, count]) => (
        <FilterPad key={label} label={label} count={count} pressed={chosen === label} onSelect={() => setChosen(label)} />
      ))}
    </div>
  );
}

/** In a row, as the filters of the Plugins tab: one chosen at a time. */
export const Inline: Story = {
  render: () => <Row />,
  play: async ({ canvas }) => {
    const [all] = canvas.getAllByRole("button", { name: "All 6" });
    const [active] = canvas.getAllByRole("button", { name: "Active 4" });
    await expect(all).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(active!);
    await expect(active).toHaveAttribute("aria-pressed", "true");
    await expect(all).toHaveAttribute("aria-pressed", "false");
  },
};

/** The chosen pad: the selection's ground, in bold. */
export const Chosen: Story = {
  args: { pressed: true },
  play: async ({ canvas }) => {
    const [pad] = canvas.getAllByRole("button", { name: "Active 4" });
    await expect(getComputedStyle(pad!).fontWeight).toBe("600");
    await expect(getComputedStyle(pad!).backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
  },
};

export const Keyboard: Story = {
  play: async ({ canvas, args }) => {
    const [pad] = canvas.getAllByRole("button", { name: "Active 4" });
    await userEvent.tab();
    await expect(pad).toHaveFocus();
    await userEvent.keyboard(" ");
    await expect(args.onSelect).toHaveBeenCalledTimes(1);
  },
};

/** In a column, as the counts of a host: a label above a 22 px value, with the status's mark. */
export const Column: Story = {
  render: () => (
    <div style={{ display: "flex", gap: 8 }}>
      <FilterPad layout="column" label="Sites" count={11} />
      <FilterPad layout="column" label="Urgent" count={1} tone="danger" pressed={false} onSelect={fn()} />
      <FilterPad layout="column" label="Need attention" count={3} tone="warn" pressed onSelect={fn()} />
      <FilterPad layout="column" label="Up to date" count={6} pressed={false} onSelect={fn()} />
    </div>
  ),
  play: async ({ canvas }) => {
    // A count without a filter is not a button, but keeps the pad's room so that the labels line up.
    await expect(canvas.queryAllByRole("button", { name: /^Sites/ })).toHaveLength(0);
    const [chosen] = canvas.getAllByRole("button", { name: /Need attention/ });
    await expect(chosen).toHaveAttribute("aria-pressed", "true");
    const [value] = canvas.getAllByText("3");
    await expect(getComputedStyle(value!.closest("[data-value]")!).fontSize).toBe("22px");
  },
};
