import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { cloudDestination, listSource } from "../../fixtures/addons";
import { AddonCard } from "./AddonCard";

const meta = {
  component: AddonCard,
  args: { addon: listSource, tab: "installed", onOpen: fn(), onLink: fn() },
  decorators: [(Story) => <div style={{ width: 280 }}><Story /></div>],
} satisfies Meta<typeof AddonCard>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Installed: Story = {
  play: async ({ canvasElement, args }) => {
    const card = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(card.getByRole("heading", { name: "Site list" })).toBeInTheDocument();
    await expect(card.getByText("by Someone")).toBeInTheDocument();
    await userEvent.click(card.getByRole("button", { name: "Open Site list" }));
    await expect(args.onOpen).toHaveBeenCalledWith("sitelist");
  },
};

// In the Library, one not installed says where to get it; Vigie installs nothing.
export const NotInstalled: Story = {
  args: { addon: cloudDestination, tab: "library" },
  play: async ({ canvasElement, args }) => {
    const card = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(card.getByText("No image of its own")).toBeInTheDocument();
    await userEvent.click(card.getByRole("link", { name: /How to install/ }));
    await expect(args.onLink).toHaveBeenCalledWith("https://example.test/vigie-cloudsave");
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  args: { tab: "library" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { name: "Liste de sites" })).toHaveLength(2);
    await expect(canvas.getAllByText("Installé")).toHaveLength(2);
  },
};
