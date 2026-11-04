import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { preview } from "../../fixtures/copies";
import { archivesAbout, CopyDialog } from "./CopyDialog";

const meta = {
  component: CopyDialog,
  args: { name: "boulangerie-exemple.ch", preview, onCopy: fn(async () => null), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 760 }}><Story /></div>],
} satisfies Meta<typeof CopyDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

const dialogOf = (canvas: ReturnType<typeof within>) =>
  within(canvas.getAllByRole("dialog", { name: "Copy boulangerie-exemple.ch to this computer" }).find((d: HTMLElement) => d.contains(document.activeElement)) ?? canvas.getAllByRole("dialog")[0]!);

// Mockups 28a and 28b: the database and wp-content chosen, two folders left out, the copy started.
export const Dialog: Story = {
  play: async ({ canvas, args }) => {
    const dialog = dialogOf(canvas);
    await expect(dialog.getByRole("button", { name: /^The database and wp-content/ })).toHaveAttribute("aria-pressed", "true");
    await expect(dialog.getByText(/^about 10 archives of 500 MB at most · 2\.22 GB to read · 214 GB free on this computer$/)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("checkbox", { name: "backups-dup-lite" }));
    await userEvent.click(dialog.getByRole("checkbox", { name: "cache" }));
    await expect(dialog.getByText(/^about 8 archives of 500 MB at most · 1\.11 GB to read/)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith("content", ["backups-dup-lite", "cache"]);
    await expect(args.onClose).toHaveBeenCalled();
  },
};

// The database alone: no folders.
export const DatabaseOnly: Story = {
  play: async ({ canvas, args }) => {
    const dialog = dialogOf(canvas);
    await userEvent.click(dialog.getByRole("button", { name: /^The database Posts/ }));
    await expect(dialog.queryByRole("checkbox")).toBeNull();
    await userEvent.click(dialog.getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith("database", []);
  },
};

// Mockup 28c: not enough room for the copy, Copy refused.
export const NoSpace: Story = {
  args: { preview: { ...preview, free: 1_400_000_000 } },
  play: async ({ canvas }) => {
    const dialog = dialogOf(canvas);
    await expect(dialog.getByRole("alert")).toHaveTextContent("Not enough space on this computer: the copy is 2.22 GB, 1.4 GB is free.");
    await expect(dialog.getByRole("button", { name: "Copy" })).toBeDisabled();
  },
};

// While the sizes are read: a wait, Copy not offered yet.
export const Reading: Story = {
  args: { preview: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("progressbar", { name: "Reading the sizes on the server…" }).length).toBeGreaterThan(0);
  },
};

// On an account without GNU find and tar, one archive per part, not split.
export const NotSplit: Story = {
  args: { preview: { ...preview, split: false } },
  play: async () => {
    await expect(archivesAbout({ ...preview, split: false }, "content", ["cache", "backups-dup-lite"])).toBe(5);
    await expect(archivesAbout(preview, "site", [])).toBe(11);
  },
};
