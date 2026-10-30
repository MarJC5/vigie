import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { addonsView } from "../../fixtures/addons";
import { AddonsPage } from "./AddonsPage";

const meta = {
  component: AddonsPage,
  args: { view: addonsView, tab: "library", onTab: fn(), onOpen: fn(), onLink: fn() },
} satisfies Meta<typeof AddonsPage>;
export default meta;
type Story = StoryObj<typeof meta>;

// The Library as a grid; the lock opens what an add-on can do, not a box in the page.
export const Library: Story = {
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getAllByRole("article")).toHaveLength(2);
    await userEvent.click(page.getByRole("tab", { name: "Installed" }));
    await expect(args.onTab).toHaveBeenCalledWith("installed");
    await userEvent.click(page.getByRole("button", { name: "A program of its own" }));
    await expect(page.getByRole("dialog", { name: "A program of its own" })).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Understood" }));
    await userEvent.click(page.getByRole("link", { name: /Write an add-on/ }));
    await expect(args.onLink).toHaveBeenCalledWith("https://github.com/MarJC5/vigie/blob/main/docs/addons.md");
  },
};

export const Installed: Story = {
  args: { tab: "installed" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await userEvent.click(page.getByRole("button", { name: "Open Site list" }));
    await expect(args.onOpen).toHaveBeenCalledWith("sitelist");
  },
};

export const NoneInstalled: Story = {
  args: { tab: "installed", view: { ...addonsView, installed: [] } },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { name: "No add-on installed" })).toBeInTheDocument();
  },
};

export const Looking: Story = {
  args: { view: null },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Looking for add-ons…")).toHaveLength(2);
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("tab", { name: "Bibliothèque" })).toHaveLength(2);
    await expect(canvas.getAllByRole("link", { name: /Écrire un add-on/ })).toHaveLength(2);
  },
};
