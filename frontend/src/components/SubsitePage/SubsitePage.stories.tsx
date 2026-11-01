import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { networkSite } from "../../fixtures/subsites";
import { SubsitePage } from "./SubsitePage";

const meta = {
  component: SubsitePage,
  args: { site: networkSite, blogId: 2, onNetwork: fn(), onOpen: fn(), now: new Date("2026-10-29T09:00:00Z") },
} satisfies Meta<typeof SubsitePage>;
export default meta;
type Story = StoryObj<typeof meta>;

// A subsite, from its network's latest scan (ADR-0029): its theme, the plugins active on it and
// those of the whole network, its administrators with their two-factor.
export const Subsite: Story = {
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { level: 1 })).toHaveTextContent("reseau-ecoles.ch/alpha");
    const row = (label: string) => page.getByText(label, { selector: "dt" }).parentElement!;
    await expect(row("Thème")).toHaveTextContent("Astra 4.8.1");
    await expect(row("Extensions actives")).toHaveTextContent("2 sur ce sous-site+ 1 sur tout le réseau");
    await expect(row("E-mail d'administration")).toHaveTextContent("direction@ecole-alpha.ch");
    const plugins = within(page.getByRole("table", { name: "Extensions" }));
    await expect(plugins.getAllByRole("row")).toHaveLength(4);
    const admins = within(page.getByRole("list", { name: "Administrateurs" }));
    await expect(admins.getByText("admin : super administrateur avec double authentification")).toBeInTheDocument();
    await expect(admins.getByText("marie : administrateur sans double authentification")).toBeInTheDocument();
    await expect(admins.getByText(/administre \/alpha et \/beta/)).toBeInTheDocument();
    await expect(admins.getByText(/administre le site principal et \/alpha/)).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "wp-admin" }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://reseau-ecoles.ch/alpha/wp-admin/");
    await userEvent.click(page.getByRole("button", { name: "reseau-ecoles.ch" }));
    await expect(args.onNetwork).toHaveBeenCalled();
  },
};

// Archived: its state, without details.
export const Archived: Story = {
  args: { blogId: 4 },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(page.getByRole("note")).toHaveTextContent("This subsite is archived: Vigie does not read its details.");
    await expect(page.queryByText("Theme", { selector: "dt" })).toBeNull();
    await expect(page.queryByRole("table")).toBeNull();
  },
};
