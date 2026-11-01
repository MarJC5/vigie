import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { HelpPage, type HelpTab } from "./HelpPage";

// Each story renders once per theme, so the page's landmarks repeat by construction; the end-to-end
// test checks the window has one of each.
const twoThemes = {
  a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } },
};

const meta = {
  component: HelpPage,
  args: { tab: "start", onTab: fn(), onOpen: fn(), onAddHost: fn() },
  parameters: twoThemes,
  // The window's content area: its padding, the page taking its height.
  decorators: [
    (Story) => (
      <div style={{ display: "flex", flexDirection: "column", padding: "24px 32px", boxSizing: "border-box" }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof HelpPage>;
export default meta;

type Story = StoryObj<typeof meta>;

export const GettingStarted: Story = {
  play: async ({ canvas, args }) => {
    const [selected] = canvas.getAllByRole("tab", { selected: true });
    await expect(selected).toHaveTextContent("Getting started");
    await expect(canvas.getAllByRole("heading", { level: 2, name: "Your first host" })).toHaveLength(2);
    // What a host that refuses answers, as the window shows it, then the cause.
    await expect(canvas.getAllByText("“The server did not answer.”")).toHaveLength(2);
    await expect(canvas.getAllByText("“The server refused the key.”")).toHaveLength(2);
    await expect(canvas.getAllByText("New Host").length).toBe(2);
    const [add] = canvas.getAllByRole("button", { name: "Add a host" });
    await userEvent.click(add!);
    await expect(args.onAddHost).toHaveBeenCalled();
    const [status] = canvas.getAllByRole("tab", { name: "Site status" });
    await userEvent.click(status!);
    await expect(args.onTab).toHaveBeenCalledWith("status");
  },
};

export const GettingStartedInFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: "Votre premier hébergement" })).toHaveLength(2);
    await expect(canvas.getAllByText("« Le serveur ne répond pas. »")).toHaveLength(2);
    await expect(canvas.getAllByText("Nouvel hébergement").length).toBe(2);
  },
};

export const UpdateTopic: Story = {
  args: { tab: "update" },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: "Update a site, and roll back" })).toHaveLength(2);
    await expect(canvas.queryAllByRole("navigation", { name: "On this page" })).toHaveLength(0);
    // Its last line leads to the full doc, on GitHub, in the browser.
    const [more] = canvas.getAllByRole("link", { name: "what Vigie writes on a server" });
    await userEvent.click(more!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://github.com/MarJC5/vigie/blob/main/docs/updates.md");
  },
};

export const StatusTopicInFrench: Story = {
  args: { tab: "status" },
  globals: { lang: "fr" },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: "Lire l'état d'un site, et ignorer un constat" })).toHaveLength(2);
    const [selected] = canvas.getAllByRole("tab", { selected: true });
    await expect(selected).toHaveTextContent("État des sites");
    const [more] = canvas.getAllByRole("link", { name: "comment les constats sont classés" });
    await userEvent.click(more!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://github.com/MarJC5/vigie/blob/main/docs/fr/security.md");
  },
};

export const ImportExportTopic: Story = {
  args: { tab: "import-export" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 3, name: "From another program" })).toHaveLength(2);
  },
};

// Every topic, in each language: its title, then its link to the full doc on GitHub.
const topic = (tab: HelpTab, lang: "en" | "fr", title: string, doc: string): Story => ({
  args: { tab },
  globals: { lang },
  play: async ({ canvas, canvasElement, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: title })).toHaveLength(2);
    const more = canvasElement.querySelector<HTMLAnchorElement>("[role=tabpanel] p:last-child a");
    await userEvent.click(more!);
    await expect(args.onOpen).toHaveBeenCalledWith(`https://github.com/MarJC5/vigie/blob/main/docs/${lang === "fr" ? "fr/" : ""}${doc}`);
  },
});

export const UpdateTopicInFrench = topic("update", "fr", "Mettre à jour un site, et revenir en arrière", "updates.md");
export const StatusTopicInEnglish = topic("status", "en", "Read a site's status, and set a finding aside", "security.md");
export const ServerTopic = topic("server", "en", "What Vigie reads on a server, and what it sends", "security.md");
export const ServerTopicInFrench = topic("server", "fr", "Ce que Vigie lit sur un serveur, et ce qu'elle envoie", "security.md");
export const ImportExportTopicInFrench = topic("import-export", "fr", "Importer et exporter", "data.md");
export const BackgroundTopic = topic("background", "en", "Keep Vigie running in the background", "background.md");
export const BackgroundTopicInFrench = topic("background", "fr", "Garder Vigie en arrière-plan", "background.md");
