import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { across } from "../../fixtures/extensions";
import { ExtensionsOverview } from "./ExtensionsOverview";

const meta = {
  component: ExtensionsOverview,
  args: { across, onSelect: fn(), onOpen: fn() },
} satisfies Meta<typeof ExtensionsOverview>;
export default meta;
type Story = StoryObj<typeof meta>;

// Each plugin or theme with how many sites run it and in which versions; a line opens to its sites.
export const AcrossSites: Story = {
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByText("1 site left out, not known yet: nouveau.example.test.").length).toBeGreaterThan(0);
    const scope = within(canvas.getAllByRole("table", { name: "Plugins and themes across the sites" })[0]!);
    const kirki = scope.getByRole("button", { name: "Sites running Kirki" });
    await expect(kirki.closest("tr")).toHaveTextContent("3 of 3");
    await expect(kirki.closest("tr")).toHaveTextContent("2 versions");
    await expect(kirki.closest("tr")).toHaveTextContent("2 sites");
    await userEvent.click(kirki);
    await expect(kirki).toHaveAttribute("aria-expanded", "true");
    await userEvent.click(scope.getAllByRole("button", { name: "cave-exemple.ch" })[0]!);
    await expect(args.onSelect).toHaveBeenCalledWith("cave");
    // A theme known only here: no link to wordpress.org.
    await expect(scope.queryByRole("link", { name: /Kirkit/ })).toBeNull();
  },
};

// The search keeps what matches, by name or slug.
export const Searched: Story = {
  play: async ({ canvas }) => {
    const [field] = canvas.getAllByRole("searchbox", { name: "Search plugins and themes" });
    await userEvent.type(field!, "kirk");
    const scope = within(canvas.getAllByRole("table")[0]!);
    await expect(scope.getAllByRole("button", { name: /^Sites running/ }).map((b) => b.getAttribute("aria-label"))).toEqual([
      "Sites running Kirki",
      "Sites running Kirkit",
    ]);
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("1 site laissé de côté, pas encore connu : nouveau.example.test.").length).toBeGreaterThan(0);
  },
};
