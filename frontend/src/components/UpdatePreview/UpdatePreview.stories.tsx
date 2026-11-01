import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { outdatedSite } from "../../fixtures/scans";
import { preview } from "../../fixtures/updates";
import { knownOf, namesOf, plannedItems, selectionOf } from "../../lib/updates";
import { UpdatePreview } from "./UpdatePreview";

const names = namesOf(outdatedSite.scan!.updates!);
const everything = plannedItems(preview, selectionOf(["core", "plugin:akismet", "plugin:woocommerce", "languages"]));

const meta = {
  component: UpdatePreview,
  args: { preview, planned: everything, names },
} satisfies Meta<typeof UpdatePreview>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithWordPress: Story = {
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getAllByRole("heading", { name: "What changes" })).toHaveLength(2);
    await expect(canvasElement).toHaveTextContent("WooCommerce9.8.1 → 10.2.0");
    await expect(canvasElement).toHaveTextContent("Folders of 2 plugins");
    await expect(canvasElement).toHaveTextContent("WordPress files, without wp-content");
    await expect(canvasElement).toHaveTextContent("~/.vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10/");
    // Updating WordPress warns of the maintenance page and of what a rollback loses.
    await expect(canvas.getAllByRole("note")).toHaveLength(2);
  },
};

export const OnePlugin: Story = {
  args: { planned: plannedItems(preview, selectionOf(["plugin:akismet"])) },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvasElement).toHaveTextContent("Folder of 1 plugin");
    await expect(canvasElement).not.toHaveTextContent("WordPress files");
    await expect(canvas.queryAllByRole("note")).toHaveLength(0);
  },
};

export const NoDumpTool: Story = {
  args: { preview: { ...preview, dumpTool: false } },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("This update cannot start");
    await expect(alert).toHaveTextContent("mariadb-dump or mysqldump");
  },
};

export const CannotLoad: Story = {
  args: { preview: { ...preview, problem: { code: "problem.cannotLoad", detail: "Error establishing a database connection" } } },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("WordPress cannot be loaded: Error establishing a database connection");
  },
};

// Each change with its changelog; an ignored update not chosen is left out, and said (ADR-0026).
const held = { ...preview, plugins: preview.plugins!.map((it) => (it.slug === "woocommerce" ? { ...it, ignored: true } : it)) };

export const Changelogs: Story = {
  args: {
    preview: held,
    planned: plannedItems(held, selectionOf(["core", "plugin:akismet", "languages"])),
    known: knownOf(outdatedSite.scan!.updates!),
    onOpen: fn(),
  },
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("link", { name: /Changelog/ })[0]!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://wordpress.org/plugins/akismet/#developers");
    await expect(canvas.getAllByRole("link", { name: /Release notes/ }).length).toBeGreaterThan(0);
    const [note] = canvas.getAllByRole("note").filter((n) => n.textContent?.includes("Left out"));
    await expect(note).toHaveTextContent("Left out: 1 ignored update");
    await expect(note).toHaveTextContent("WooCommerce 9.8.1 → 10.2.0. Tick it in the Updates tab to install it this time.");
  },
};
