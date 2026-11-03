import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import type { Journal } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { siteChanges } from "../../fixtures/changes";
import { sampleTree } from "../../fixtures/tree";
import { ActivityPage } from "./ActivityPage";

// Fictional: today is 31 October 2026, local time.
const now = new Date(2026, 9, 31, 14, 0);
const at = (day: number, h: number, m = 0) => new Date(2026, 9, day, h, m).toISOString();

const journal: Journal = {
  since: at(1, 14),
  entries: [
    { kind: "run", at: at(31, 10, 12), ended: at(31, 10, 13), by: "user", path: "Clients/Shared hosting", siteId: "", sites: 3, outcome: "completed", summary: "", wpCli: null, changes: [], siteChanges: [], notices: [] },
    {
      kind: "run", at: at(31, 3), ended: at(31, 3, 4), by: "schedule", path: "", siteId: "", sites: 4, outcome: "completed", summary: "", wpCli: null,
      changes: [
        { kind: "urgent", nodeId: "boulangerie", name: "boulangerie-exemple.ch" },
        { kind: "recovered", nodeId: "atelier", name: "atelier-exemple.ch" },
      ],
      siteChanges: [{ siteId: "boulangerie", name: "boulangerie-exemple.ch", changes: siteChanges.slice(0, 5) }],
      notices: [
        { kind: "urgent", names: ["boulangerie-exemple.ch"], text: "" },
        { kind: "admin", names: ["boulangerie-exemple.ch"], text: "" },
        { kind: "installed", names: ["boulangerie-exemple.ch"], text: "" },
      ],
    },
    { kind: "update", at: at(30, 16, 40), ended: at(30, 16, 42), by: "", path: "Clients/Shared hosting/boulangerie-exemple.ch", siteId: "boulangerie", sites: 0, outcome: "completed", summary: "Akismet 5.3 → 5.7.2", wpCli: null, changes: [], siteChanges: [], notices: [] },
    { kind: "run", at: at(28, 9, 5), ended: at(28, 9, 6), by: "cli", path: "Personal/Managed VPS", siteId: "", sites: 1, outcome: "cancelled", summary: "", wpCli: null, changes: [], siteChanges: [], notices: [] },
  ],
};

const meta = {
  component: ActivityPage,
  args: { journal, days: 30, filter: "", nodes: sampleTree, onFilter: fn(), onOlder: fn(), onOpen: fn(), now },
} satisfies Meta<typeof ActivityPage>;
export default meta;
type Story = StoryObj<typeof meta>;

// By day, newest first; each scan says who started it and what it changed; a name opens its site.
export const WhatChanged: Story = {
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { level: 2, name: "Aujourd'hui" })).toBeInTheDocument();
    await expect(page.getByRole("heading", { level: 2, name: "Hier" })).toBeInTheDocument();
    await expect(page.getByText("Relevé automatique de tous les sites")).toBeInTheDocument();
    await expect(page.getByText("Notifié : boulangerie-exemple.ch")).toBeInTheDocument();
    await userEvent.click(page.getAllByRole("button", { name: "boulangerie-exemple.ch" })[0]!);
    await expect(args.onOpen).toHaveBeenCalledWith("boulangerie");
    await expect(page.getByText("1 site · arrêté")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Afficher plus ancien" }));
    await expect(args.onOlder).toHaveBeenCalled();
  },
};

// Under a run, what changed on each site since its previous scan (ADR-0027), its name opening it;
// the changes that notified said on one line.
export const SiteChanges: Story = {
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    const list = within(page.getByRole("list", { name: "Changements de boulangerie-exemple.ch" }));
    await expect(list.getAllByRole("img", { name: "À regarder" })).toHaveLength(2);
    await expect(list.getAllByRole("listitem").map((li) => li.textContent)).toEqual([
      "Nouvel administrateur\u00a0: marie.dupont",
      "Administrateur retiré\u00a0: stagiaire",
      "Extension installée hors de Vigie\u00a0: WP File Manager 7.2.1",
      "Thème actif\u00a0: Twenty Twenty-Four → Astra",
      "WordPress 7.1.3 → 7.1.4, hors de Vigie",
    ]);
    await expect(page.getByText("Notifié : nouvel administrateur et extension ou thème installé hors de Vigie sur boulangerie-exemple.ch")).toBeInTheDocument();
    await userEvent.click(page.getAllByRole("button", { name: "boulangerie-exemple.ch" })[1]!);
    await expect(args.onOpen).toHaveBeenCalledWith("boulangerie");
  },
};

// Nothing in the span: said so, and older days still within reach.
export const Nothing: Story = {
  args: { journal: { since: at(1, 14), entries: [] } },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(page.getByText("Nothing in the last 30 days.")).toBeInTheDocument();
    await userEvent.selectOptions(page.getByRole("combobox", { name: "Show" }), "shared");
    await expect(args.onFilter).toHaveBeenCalledWith("shared");
  },
};

// The end of a long run notified (ADR-0035): under the run, or under the last update of a run over
// several sites, what its notification said.
export const TheEndOfALongRun: Story = {
  globals: { lang: "fr" },
  args: {
    journal: {
      since: at(1, 14),
      entries: [
        {
          kind: "run", at: at(31, 13, 0), ended: at(31, 13, 2), by: "user", path: "", siteId: "", sites: 12, outcome: "completed", summary: "", wpCli: null,
          changes: [{ kind: "urgent", nodeId: "boulangerie", name: "boulangerie-exemple.ch" }], siteChanges: [],
          notices: [{ kind: "ended", names: ["boulangerie-exemple.ch"], text: "Relevé terminé, 12 sites relevés, 1 urgent. boulangerie-exemple.ch est devenu urgent." }],
        },
        {
          kind: "update", at: at(31, 11, 5), ended: at(31, 11, 12), by: "", path: "Clients/Shared hosting/boulangerie-exemple.ch", siteId: "boulangerie", sites: 0,
          outcome: "completed", summary: "Akismet 5.3 → 5.7.2", wpCli: null, changes: [], siteChanges: [],
          notices: [{ kind: "ended", names: [], text: "Mise à jour terminée, 5 sites mis à jour, 1 en échec." }],
        },
      ],
    },
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Notifié : Relevé terminé, 12 sites relevés, 1 urgent. boulangerie-exemple.ch est devenu urgent.")).toBeInTheDocument();
    await expect(page.getByText("Notifié : Mise à jour terminée, 5 sites mis à jour, 1 en échec.")).toBeInTheDocument();
  },
};

// The installs of WP-CLI (ADR-0039): under their host, which opens; one for a host never added
// under the name it had; what was installed, or where it stopped.
export const WPCLIInstalls: Story = {
  globals: { lang: "fr" },
  args: {
    journal: {
      since: at(1, 14),
      entries: [
        {
          kind: "wpcli", at: at(31, 14, 38), ended: at(31, 14, 38), by: "", path: "Clients/Shared hosting", siteId: "", sites: 0, outcome: "completed", summary: "",
          wpCli: { hostId: "shared", version: "2.12.0", command: "~/bin/wp", failedStep: "", message: "" }, changes: [], siteChanges: [], notices: [],
        },
        {
          kind: "wpcli", at: at(31, 14, 31), ended: at(31, 14, 31), by: "", path: "Nouveau VPS", siteId: "", sites: 0, outcome: "failed", summary: "",
          wpCli: { hostId: "", version: "2.12.0", command: "", failedStep: "send", message: "Connection to ssh.example-host.ch closed by remote host." }, changes: [], siteChanges: [], notices: [],
        },
      ],
    },
  },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("WP-CLI 2.12.0 dans ~/bin/wp, vérifié (SHA-512). Commande de l'hébergement : ~/bin/wp.")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Clients/Shared hosting" }));
    await expect(args.onOpen).toHaveBeenCalledWith("shared");
    await expect(page.getByText("Installation de WP-CLI sur Nouveau VPS")).toBeInTheDocument();
    await expect(page.queryByRole("button", { name: "Nouveau VPS" })).toBeNull();
    await expect(page.getByText("Arrêtée à l'étape de l'envoi : Connection to ssh.example-host.ch closed by remote host.")).toBeInTheDocument();
    await expect(page.getByText("en échec")).toBeInTheDocument();
  },
};
