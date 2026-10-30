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
    theme: "system",
    onTheme: fn(),
    lookups,
    onAgree: fn(),
    onWithdraw: fn(),
    onDestination: fn(),
    version: "0.5.0",
    onOpen: fn(),
    onExport: fn(),
    onImport: fn(),
    background: { on: false, autoScan: 0, notify: false },
    onBackground: fn(),
    loginItem: { supported: true, on: false },
    onLoginItem: fn(),
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
    await expect(page.getByRole("combobox", { name: "Appearance" })).toHaveValue("system");
    await userEvent.selectOptions(page.getByRole("combobox", { name: "Appearance" }), "light");
    await expect(args.onTheme).toHaveBeenCalledWith("light");
  },
};

// Everything off by default; start at login waits for the background mode.
export const BackgroundOff: Story = {
  args: { tab: "general" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const login = page.getByRole("switch", { name: "Open at login" });
    await expect(login).toBeDisabled();
    await expect(page.getByText("Needs Vigie to keep running.")).toBeInTheDocument();
    await expect(page.getByRole("combobox", { name: "Automatic scans" })).toHaveValue("0");
    await userEvent.click(page.getByRole("switch", { name: "Keep Vigie running" }));
    await expect(args.onBackground).toHaveBeenCalledWith({ on: true, autoScan: 0, notify: false });
    await userEvent.selectOptions(page.getByRole("combobox", { name: "Automatic scans" }), "6");
    await expect(args.onBackground).toHaveBeenCalledWith({ on: false, autoScan: 6, notify: false });
  },
};

// Kept running, at login, every 6 hours, with notifications.
export const BackgroundOn: Story = {
  args: { tab: "general", background: { on: true, autoScan: 6, notify: true }, loginItem: { supported: true, on: true } },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    const login = page.getByRole("switch", { name: "Ouvrir à l'ouverture de session" });
    await expect(login).toHaveAttribute("aria-checked", "true");
    await expect(page.getByRole("combobox", { name: "Relevés automatiques" })).toHaveValue("6");
    await userEvent.click(login);
    await expect(args.onLoginItem).toHaveBeenCalledWith(false);
  },
};

// Where the system has no login items, the switch is not offered.
export const NoLoginItems: Story = {
  args: { tab: "general", loginItem: { supported: false, on: false } },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.queryByRole("switch", { name: "Open at login" })).toBeNull();
  },
};

// Export, and the three sources of an import; what the last one did under its row.
export const Data: Story = {
  args: { tab: "data", dataNote: { row: "import", text: "5 folders, hosts and sites added." } },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.click(page.getByRole("button", { name: "Export…" }));
    await expect(args.onExport).toHaveBeenCalled();
    await userEvent.click(page.getByRole("button", { name: "Choose…" }));
    await expect(args.onImport).toHaveBeenCalledWith("ssh");
    await userEvent.click(page.getAllByRole("button", { name: "Import…" })[1]!);
    await expect(args.onImport).toHaveBeenLastCalledWith("csv");
    await expect(page.getByText("5 folders, hosts and sites added.")).toBeInTheDocument();
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
