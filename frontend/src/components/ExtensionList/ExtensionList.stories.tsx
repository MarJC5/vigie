import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { extensions } from "../../fixtures/extensions";
import { networkSite } from "../../fixtures/subsites";
import { ExtensionList } from "./ExtensionList";

const meta = {
  component: ExtensionList,
  args: { extensions, onFinding: fn(), onOpen: fn() },
} satisfies Meta<typeof ExtensionList>;
export default meta;
type Story = StoryObj<typeof meta>;

// The slugs of a table's lines, in order.
const slugs = (table: HTMLElement) => within(table).getAllByRole("row").slice(1).map((r) => r.querySelector("[data-selectable]")!.textContent);

// The plugins, then the themes; each filter with its count.
export const Plugins: Story = {
  play: async ({ canvas, args }) => {
    const [filters] = canvas.getAllByRole("group", { name: "Show" });
    await expect(within(filters!).getAllByRole("button").map((b) => b.textContent)).toEqual(["All6", "Active4", "Inactive2", "With an update2"]);
    const [plugins] = canvas.getAllByRole("table", { name: "Plugins" });
    await expect(plugins).toHaveTextContent("Must-use");
    await expect(plugins).toHaveTextContent("→ 9.8.5");
    await expect(plugins).toHaveTextContent("Not on wordpress.org");
    const [themes] = canvas.getAllByRole("table", { name: "Themes" });
    await expect(themes).toHaveTextContent("Parent");
    // A plugin wordpress.org knows opens its page there; its worst finding opens the Security tab.
    await userEvent.click(within(plugins!).getByRole("link", { name: /^Akismet/ }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://wordpress.org/plugins/akismet/");
    await userEvent.click(within(plugins!).getByRole("button", { name: /SQL injection/ }));
    await expect(args.onFinding).toHaveBeenCalled();
  },
};

// With an update: two plugins, no theme.
export const WithAnUpdate: Story = {
  play: async ({ canvas }) => {
    // In the first of the two themes, where the filter is chosen.
    const list = within(canvas.getAllByRole("group", { name: "Show" })[0]!.parentElement!);
    const update = list.getByRole("button", { name: /With an update/ });
    await userEvent.click(update);
    await expect(update).toHaveAttribute("aria-pressed", "true");
    await expect(slugs(list.getByRole("table", { name: "Plugins" }))).toEqual(["woocommerce", "akismet"]);
    await expect(list.queryAllByRole("table", { name: "Themes" })).toHaveLength(0);
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    const [plugins] = canvas.getAllByRole("table", { name: "Extensions" });
    await expect(plugins).toHaveTextContent("Hors wordpress.org");
    await expect(canvas.getAllByRole("table", { name: "Thèmes" })[0]).toHaveTextContent("Inactif");
  },
};

// An update ignored says so on its line, in gray (ADR-0026).
export const AnIgnoredUpdate: Story = {
  args: {
    extensions: [
      ...extensions,
      { ...extensions[0]!, slug: "page-builder-kit", title: "Page Builder Kit", version: "4.2.0", updateVersion: "5.0.0", autoUpdate: false, worst: null,
        ignore: { untilVersion: "", reason: "the theme uses its internal API", created: "2026-11-01T10:00:00Z" } },
    ],
  },
  play: async ({ canvas }) => {
    const [table] = canvas.getAllByRole("table", { name: "Plugins" });
    const row = within(table!).getByRole("row", { name: /Page Builder Kit/ });
    await expect(row).toHaveTextContent("→ 5.0.0 · ignored");
  },
};

// On a network (ADR-0029): where each plugin and theme is active; a line opens to its subsites,
// each opening its page.
export const OnANetwork: Story = {
  args: { extensions: networkSite.scan!.extensions!, subsites: networkSite.subsites!, onOpenSubsite: fn() },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    const plugins = within(page.getByRole("table", { name: "Plugins" }));
    await expect(plugins.getByRole("columnheader", { name: "Activation" })).toBeInTheDocument();
    await expect(plugins.getAllByRole("row").map((r) => r.children[2]?.textContent)).toEqual([
      "Activation", "Across the network", "On 2 subsites", "On 1 subsite", "Nowhere",
    ]);
    await userEvent.click(plugins.getByRole("button", { name: "On 2 subsites" }));
    const list = within(plugins.getByRole("list", { name: "Subsites where Contact Form 7 is active" }));
    await userEvent.click(list.getByRole("button", { name: "/beta" }));
    await expect(args.onOpenSubsite).toHaveBeenCalledWith(3);
    // Active is active anywhere on the network.
    const [filters] = page.getAllByRole("group", { name: "Show" });
    await expect(within(filters!).getAllByRole("button").map((b) => b.textContent)).toEqual(["All6", "Active5", "Inactive1", "With an update0"]);
  },
};
