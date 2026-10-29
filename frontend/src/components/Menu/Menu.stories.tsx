import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { Menu, type MenuEntry } from "./Menu";

const entries: MenuEntry[] = [
  { kind: "item", id: "scan", label: "Scan host", icon: "refresh", shortcut: "⌘R", keyshortcuts: "Meta+R" },
  { kind: "separator" },
  { kind: "item", id: "rename", label: "Rename", icon: "rename", shortcut: "↩" },
  { kind: "item", id: "move", label: "Move to...", icon: "move" },
  {
    kind: "colors",
    label: "Color",
    value: "none",
    names: { none: "None", red: "Red", green: "Green", blue: "Blue", yellow: "Yellow", cyan: "Cyan", magenta: "Magenta", orange: "Orange" },
  },
  { kind: "item", id: "find", label: "Find new sites...", icon: "findSites", disabled: true },
  { kind: "separator" },
  { kind: "item", id: "delete", label: "Delete the host...", icon: "trash", danger: true, shortcut: "⌘⌫" },
];

const meta = {
  component: Menu,
  args: { label: "Actions on Shared hosting", entries, x: 16, y: 16, onSelect: fn(), onColor: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 380 }}><Story /></div>],
} satisfies Meta<typeof Menu>;
export default meta;
type Story = StoryObj<typeof meta>;

export const HostMenu: Story = {
  play: async ({ canvas, args }) => {
    const [menu] = canvas.getAllByRole("menu", { name: "Actions on Shared hosting" });
    const items = within(menu!).getAllByRole("menuitem");
    await expect(items.map((i) => i.textContent)).toEqual([
      "Scan host⌘R", "Rename↩", "Move to...", "Find new sites...", "Delete the host...⌘⌫",
    ]);
    await expect(items[0]).toHaveAttribute("aria-keyshortcuts", "Meta+R");
    await expect(items[3]).toHaveAttribute("aria-disabled", "true");
    await userEvent.click(items[2]!);
    await expect(args.onSelect).toHaveBeenCalledWith("move");
    await userEvent.click(items[3]!);
    await expect(args.onSelect).toHaveBeenCalledTimes(1);
  },
};

// The keyboard moves over the items and the swatches, skipping what is disabled.
export const Keyboard: Story = {
  play: async ({ canvas, args }) => {
    const [menu] = canvas.getAllByRole("menu");
    within(menu!).getAllByRole("menuitem")[0]!.focus();
    await userEvent.keyboard("{ArrowDown}");
    await expect(within(menu!).getByRole("menuitem", { name: "Rename" })).toHaveFocus();
    await userEvent.keyboard("{End}");
    await expect(within(menu!).getByRole("menuitem", { name: "Delete the host..." })).toHaveFocus();
    await userEvent.keyboard("{ArrowUp}");
    await expect(within(menu!).getByRole("menuitemradio", { name: "Orange" })).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await expect(args.onColor).toHaveBeenCalledWith("orange");
    await userEvent.keyboard("{Escape}");
    await expect(args.onClose).toHaveBeenCalled();
  },
};

export const Colors: Story = {
  play: async ({ canvas, args }) => {
    const [menu] = canvas.getAllByRole("menu");
    const swatches = within(menu!).getAllByRole("menuitemradio");
    await expect(swatches).toHaveLength(8);
    await expect(within(menu!).getByRole("menuitemradio", { name: "None" })).toHaveAttribute("aria-checked", "true");
    await userEvent.click(within(menu!).getByRole("menuitemradio", { name: "Green" }));
    await expect(args.onColor).toHaveBeenCalledWith("green");
  },
};

export const ClosesOutside: Story = {
  play: async ({ canvasElement, args }) => {
    await userEvent.click(canvasElement.ownerDocument.body);
    await expect(args.onClose).toHaveBeenCalled();
  },
};
