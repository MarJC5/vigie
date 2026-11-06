import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import type { SelfUpdateView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
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
    database: { bytes: 287_400_000, free: 112_300_000, problem: null },
    onCompact: fn(),
    onBackupDays: fn(),
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
    await userEvent.click(page.getByRole("button", { name: "Export" }));
    await expect(args.onExport).toHaveBeenCalled();
    await userEvent.click(page.getByRole("button", { name: "Choose" }));
    await expect(args.onImport).toHaveBeenCalledWith("ssh");
    await userEvent.click(page.getAllByRole("button", { name: "Import" })[1]!);
    await expect(args.onImport).toHaveBeenLastCalledWith("csv");
    await expect(page.getByText("5 folders, hosts and sites added.")).toBeInTheDocument();
  },
};

// The history kept and the database's size; compacting it is offered.
export const Database: Story = {
  args: { tab: "data" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("History kept")).toBeInTheDocument();
    await expect(page.getByText("Size: 287.4 MB, 112.3 MB of it free")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Compact" }));
    await expect(args.onCompact).toHaveBeenCalled();
  },
};

// How long the backups of an update stay on the servers (ADR-0033): 30 days by default.
export const BackupsKept: Story = {
  args: { tab: "data" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const select = page.getByRole("combobox", { name: "Backups before updates" });
    await expect(select).toHaveValue("30");
    await userEvent.selectOptions(select, "No limit");
    await expect(args.onBackupDays).toHaveBeenCalledWith(0);
  },
};

// While it is compacted, and while scans run: the button waits.
export const Compacting: Story = {
  args: { tab: "data", compacting: true },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("button", { name: "Compacting…" })).toBeDisabled();
  },
};

export const CompactWhileScanning: Story = {
  args: { tab: "data", busy: true },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("button", { name: "Compact" })).toBeDisabled();
  },
};

// Compacted: the new size, and what it was.
export const Compacted: Story = {
  args: {
    tab: "data",
    database: { bytes: 175_100_000, free: 0, problem: null },
    dataNote: { row: "compact", text: "Compacted: 287.4 MB → 175.1 MB" },
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Size: 175.1 MB")).toBeInTheDocument();
    await expect(page.getByText("Compacted: 287.4 MB → 175.1 MB")).toBeInTheDocument();
  },
};

export const DatabaseInFrench: Story = {
  args: { tab: "data" },
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText(/^Taille\s: 287,4\sMo, dont 112,3\sMo libres$/)).toHaveLength(2);
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

// The vigie command not linked yet (ADR-0036): the one command that links it, to copy, and a check
// once it ran.
export const CommandNotLinked: Story = {
  args: {
    tab: "general",
    command: {
      linked: false, appImage: false, path: "/usr/local/bin/vigie",
      command: "sudo ln -sf '/Applications/Vigie.app/Contents/MacOS/Vigie' '/usr/local/bin/vigie'",
    },
    onCheckCommand: fn(),
    onCopy: fn(),
  },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { level: 2, name: "Command line" })).toBeInTheDocument();
    await expect(page.getByText("To scan, update or export from Terminal, run this command there once. It asks for your password.")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith("sudo ln -sf '/Applications/Vigie.app/Contents/MacOS/Vigie' '/usr/local/bin/vigie'");
    await userEvent.click(page.getByRole("button", { name: "Check again" }));
    await expect(args.onCheckCommand).toHaveBeenCalled();
  },
};

// Linked: where from, nothing to run.
export const CommandLinked: Story = {
  args: { tab: "general", command: { linked: true, appImage: false, path: "/usr/local/bin/vigie", command: "" } },
  globals: { lang: "fr" },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText(/Liée depuis \/usr\/local\/bin\/vigie\./)).toBeInTheDocument();
    await expect(page.getByText("Installée")).toBeInTheDocument();
    await expect(page.queryByRole("button", { name: "Copier" })).toBeNull();
  },
};

// An AppImage links into ~/.local/bin, without a password.
export const CommandAppImage: Story = {
  args: {
    tab: "general",
    command: {
      linked: false, appImage: true, path: "/home/marie/.local/bin/vigie",
      command: "mkdir -p '/home/marie/.local/bin' && ln -sf '/home/marie/Applications/Vigie-x86_64.AppImage' '/home/marie/.local/bin/vigie'",
    },
    onCopy: fn(),
  },
  globals: { lang: "fr" },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText(/~\/\.local\/bin doit être dans votre PATH\./)).toBeInTheDocument();
  },
};

// Vigie's own update (ADR-0038): a view as the Go side sends it.
const selfUpdate = (over: Partial<SelfUpdateView>): SelfUpdateView => ({
  install: "app", current: "0.5.0", state: "idle", latest: "", notes: "", published: "", checked: "", written: 0, total: 0, problem: null, ...over,
});

// A new version: its notes, the link to all of them, Install and restart.
export const UpdateAvailable: Story = {
  args: {
    tab: "about", onInstallUpdate: fn(),
    selfUpdate: selfUpdate({
      state: "available", latest: "0.6.0", published: "2026-11-14T09:00:00Z",
      notes: "- Several items selected in the site list\n- A notification when a long scan ends\n- The vigie command outside the app",
    }),
  },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Vigie 0.5.0. Version 0.6.0 is available, published on 14.11.2026.")).toBeInTheDocument();
    await expect(page.getByText("What is new in 0.6.0")).toBeInTheDocument();
    await expect(page.getByText("A notification when a long scan ends")).toBeInTheDocument();
    await userEvent.click(page.getByRole("link", { name: /All notes on GitHub/ }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://github.com/MarJC5/vigie/releases/tag/v0.6.0");
    await userEvent.click(page.getByRole("button", { name: "Install and restart" }));
    await expect(args.onInstallUpdate).toHaveBeenCalled();
  },
};

// Downloading, the bar counting the bytes.
export const UpdateDownloading: Story = {
  args: { tab: "about", selfUpdate: selfUpdate({ state: "downloading", latest: "0.6.0", written: 11_200_000, total: 18_000_000 }) },
  globals: { lang: "fr" },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("progressbar")).toHaveAttribute("aria-valuenow", "11200000");
    await expect(page.getByRole("button", { name: "Téléchargement…" })).toBeDisabled();
  },
};

// Up to date, checked; a check on demand.
export const UpToDate: Story = {
  args: { tab: "about", onCheckUpdate: fn(), selfUpdate: selfUpdate({ state: "up-to-date", checked: "2026-10-30T14:32:00Z" }) },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText(/Vigie 0\.5\.0, checked on 30\.10\.2026 at/)).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Check for updates" }));
    await expect(args.onCheckUpdate).toHaveBeenCalled();
  },
};

// A signature refused: the file is discarded, and the user can try again.
export const UpdateRefused: Story = {
  args: {
    tab: "about", onInstallUpdate: fn(),
    selfUpdate: selfUpdate({ state: "error", latest: "0.6.0", problem: { code: "problem.selfUpdateSignature", detail: "" } }),
  },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Sa signature ne correspond pas à la clé de Vigie : le fichier téléchargé est écarté.")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Réessayer" }));
    await expect(args.onInstallUpdate).toHaveBeenCalled();
  },
};

// Homebrew updates it: the command to copy; a Linux package and a build of the sources say so.
export const UpdatedElsewhere: Story = {
  args: { tab: "about", onCopy: fn(), selfUpdate: selfUpdate({ install: "homebrew" }) },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText(/installée avec Homebrew/)).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Copier" }));
    await expect(args.onCopy).toHaveBeenCalledWith("brew upgrade --cask vigie");
    await expect(page.queryByRole("button", { name: "Rechercher une mise à jour" })).toBeNull();
  },
};

// github.com in a section of its own, its switch.
export const LookupsWithGitHub: Story = {
  args: { lookups: { ...lookups, destinations: [...lookups.destinations, { id: "github", on: true }] } },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { level: 2, name: "Nouvelles versions de Vigie" })).toBeInTheDocument();
    await expect(page.getByText(/une fois par jour au plus, et quand vous le demandez dans À propos/)).toBeInTheDocument();
    await userEvent.click(page.getByRole("switch", { name: "github.com" }));
    await expect(args.onDestination).toHaveBeenCalledWith("github", false);
  },
};

// Updates of the sites (ADR-0042): plugins and themes roll back on their own, on by default.
export const RollbackOnItsOwn: Story = {
  args: { tab: "general", autoRollback: true, onAutoRollback: fn() },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { name: "Updates of the sites" })).toBeInTheDocument();
    const toggle = page.getByRole("switch", { name: "Roll back on its own" });
    await expect(toggle).toHaveAttribute("aria-checked", "true");
    await userEvent.click(toggle);
    await expect(args.onAutoRollback).toHaveBeenCalledWith(false);
  },
};

const linkedConfig = '{\n  "mcpServers": {\n    "vigie": {\n      "command": "/usr/local/bin/vigie",\n      "args": ["mcp"]\n    }\n  }\n}';

// The assistants off, the default (ADR-0043): the three levels, what is never allowed, the lines.
export const AssistantsOff: Story = {
  args: { tab: "assistants", assistants: { access: "off", config: linkedConfig }, onAssistantAccess: fn(), onCopy: fn(), onAssistantsHelp: fn() },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("tab", { name: "Assistants" })).toHaveAttribute("aria-selected", "true");
    const levels = within(page.getByRole("group", { name: "What an assistant may do" }));
    await expect(levels.getByRole("button", { name: /^Off/ })).toHaveAttribute("aria-pressed", "true");
    await expect(page.queryByText("Let your client ask you")).toBeNull();
    await expect(page.getByText(/The content of debug\.log and your notes are never sent\./)).toBeInTheDocument();
    await userEvent.click(levels.getByRole("button", { name: /^Read/ }));
    await expect(args.onAssistantAccess).toHaveBeenCalledWith("read");
    await userEvent.click(page.getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith(linkedConfig);
    await userEvent.click(page.getByRole("button", { name: "Help: connect a client" }));
    await expect(args.onAssistantsHelp).toHaveBeenCalled();
  },
};

// Read: the sites, the journal and a scan.
export const AssistantsRead: Story = {
  args: { tab: "assistants", assistants: { access: "read", config: linkedConfig }, onAssistantAccess: fn() },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    const levels = within(page.getByRole("group", { name: "Ce qu'un assistant peut faire" }));
    await expect(levels.getByRole("button", { name: /^Lecture/ })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(levels.getByRole("button", { name: /^Actions/ }));
    await expect(args.onAssistantAccess).toHaveBeenCalledWith("act");
  },
};

// Actions: the notice that the client must keep asking.
export const AssistantsAct: Story = {
  args: { tab: "assistants", assistants: { access: "act", config: linkedConfig }, onAssistantAccess: fn() },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("button", { name: /^Actions/ })).toHaveAttribute("aria-pressed", "true");
    await expect(page.getByText("Let your client ask you")).toBeInTheDocument();
    await expect(page.getByText(/"command": "\/usr\/local\/bin\/vigie"/)).toBeInTheDocument();
  },
};

// The command not linked: no lines, the way to General.
export const AssistantsNoCommand: Story = {
  args: { tab: "assistants", assistants: { access: "act", config: "" }, onAssistantAccess: fn() },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("The vigie command is not linked")).toBeInTheDocument();
    await expect(page.queryByRole("button", { name: "Copy" })).toBeNull();
    await userEvent.click(page.getByRole("button", { name: "Open General" }));
    await expect(args.onTab).toHaveBeenCalledWith("general");
  },
};
