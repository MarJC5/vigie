import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { SettingsPage } from "./SettingsPage";

const lookups = {
  agreed: "2026-10-30T14:12:00Z",
  asked: "",
  destinations: [
    { id: "wpvulnerability", on: true },
    { id: "wordpress.org", on: true },
    { id: "registry", on: false },
    { id: "site", on: true },
  ],
};

const meta = {
  component: SettingsPage,
  args: {
    tab: "lookups",
    onTab: fn(),
    language: "en",
    onLanguage: fn(),
    lookups,
    onAgree: fn(),
    onWithdraw: fn(),
    onDestination: fn(),
    version: "0.5.0",
    onOpen: fn(),
  },
} satisfies Meta<typeof SettingsPage>;
export default meta;
type Story = StoryObj<typeof meta>;

// Each service with what it receives and its switch; one turned off says what is not checked.
export const LookupsOutside: Story = {
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getAllByRole("switch")).toHaveLength(4);
    await expect(page.getByText("Off: domain expiry is not checked.")).toBeInTheDocument();
    await expect(page.getByText("Receives: plugin slugs · answer kept 1 day")).toBeInTheDocument();
    await userEvent.click(page.getByRole("switch", { name: "Domain registry (.com, .fr…)" }));
    await expect(args.onDestination).toHaveBeenCalledWith("registry", true);
    await expect(page.getByText(/Given on 30\.10\.2026 at/)).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Withdraw consent" }));
    await expect(args.onWithdraw).toHaveBeenCalled();
    await userEvent.click(page.getByRole("tab", { name: "About" }));
    await expect(args.onTab).toHaveBeenCalledWith("about");
  },
};

export const NotAgreed: Story = {
  args: { lookups: { ...lookups, agreed: "" } },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await userEvent.click(page.getByRole("button", { name: "Agree" }));
    await expect(args.onAgree).toHaveBeenCalled();
  },
};

export const General: Story = {
  args: { tab: "general" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.selectOptions(page.getByRole("combobox", { name: "Language" }), "fr");
    await expect(args.onLanguage).toHaveBeenCalledWith("fr");
  },
};

export const About: Story = {
  args: { tab: "about" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Vigie 0.5.0")).toBeInTheDocument();
    await expect(page.getByText("MIT. Not affiliated with the WordPress Foundation.")).toBeInTheDocument();
    await userEvent.click(page.getByRole("link", { name: /wpvulnerability\.com/ }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://www.wpvulnerability.com/");
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("tab", { name: "Recherches extérieures" })).toHaveLength(2);
    await expect(canvas.getAllByText(/^Reçoit\s: slugs des extensions · réponse gardée 1 jour$/)).toHaveLength(2);
  },
};
