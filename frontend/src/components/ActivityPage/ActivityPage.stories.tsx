import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import type { Journal } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sampleTree } from "../../fixtures/tree";
import { ActivityPage } from "./ActivityPage";

// Fictional: today is 31 October 2026, local time.
const now = new Date(2026, 9, 31, 14, 0);
const at = (day: number, h: number, m = 0) => new Date(2026, 9, day, h, m).toISOString();

const journal: Journal = {
  since: at(1, 14),
  entries: [
    { kind: "run", at: at(31, 10, 12), ended: at(31, 10, 13), by: "user", path: "Clients/Shared hosting", siteId: "", sites: 3, outcome: "completed", summary: "", changes: [], siteChanges: [], notices: [] },
    {
      kind: "run", at: at(31, 3), ended: at(31, 3, 4), by: "schedule", path: "", siteId: "", sites: 4, outcome: "completed", summary: "",
      changes: [
        { kind: "urgent", nodeId: "boulangerie", name: "boulangerie-exemple.ch" },
        { kind: "recovered", nodeId: "atelier", name: "atelier-exemple.ch" },
      ],
      siteChanges: [],
      notices: [{ kind: "urgent", names: ["boulangerie-exemple.ch"] }],
    },
    { kind: "update", at: at(30, 16, 40), ended: at(30, 16, 42), by: "", path: "Clients/Shared hosting/boulangerie-exemple.ch", siteId: "boulangerie", sites: 0, outcome: "completed", summary: "Akismet 5.3 → 5.7.2", changes: [], siteChanges: [], notices: [] },
    { kind: "run", at: at(28, 9, 5), ended: at(28, 9, 6), by: "cli", path: "Personal/Managed VPS", siteId: "", sites: 1, outcome: "cancelled", summary: "", changes: [], siteChanges: [], notices: [] },
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
    await userEvent.click(page.getByRole("button", { name: "boulangerie-exemple.ch" }));
    await expect(args.onOpen).toHaveBeenCalledWith("boulangerie");
    await expect(page.getByText("1 site · arrêté")).toBeInTheDocument();
    await userEvent.click(page.getByRole("button", { name: "Afficher plus ancien" }));
    await expect(args.onOlder).toHaveBeenCalled();
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
