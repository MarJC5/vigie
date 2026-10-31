import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { HelpPage } from "./HelpPage";

// Each story renders once per theme, so the page's landmarks repeat by construction; the end-to-end
// test checks the window has one of each.
const twoThemes = {
  a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } },
};

const meta = {
  component: HelpPage,
  args: { tab: "start", onTab: fn(), onOpen: fn(), onAddHost: fn() },
  parameters: twoThemes,
  // The window's content area: its padding, its height, and the page filling it.
  decorators: [
    (Story) => (
      <div style={{ height: 760, display: "flex", flexDirection: "column", padding: "24px 32px", boxSizing: "border-box" }}>
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
    const [security] = canvas.getAllByRole("tab", { name: "Security" });
    await userEvent.click(security!);
    await expect(args.onTab).toHaveBeenCalledWith("security");
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

export const SecurityDoc: Story = {
  args: { tab: "security" },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: /Security scans/ })).toHaveLength(2);
    // The doc's sections beside it, the first one read.
    const [onPage] = canvas.getAllByRole("navigation", { name: "On this page" });
    await expect(onPage).toHaveTextContent("How findings are ranked");
    const [first] = canvas.getAllByRole("link", { name: "What is read on the server" });
    await expect(first).toHaveAttribute("aria-current", "location");
    // A link to a site opens in the browser.
    const [link] = canvas.getAllByRole("link", { name: "WPVulnerability" });
    await userEvent.click(link!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://www.wpvulnerability.com/");
  },
};

export const BackgroundDocInFrench: Story = {
  args: { tab: "background" },
  globals: { lang: "fr" },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 2, name: "Vigie en arrière-plan" })).toHaveLength(2);
    // A link to another doc opens its tab.
    const [doc] = canvas.getAllByRole("link", { name: "security.md" });
    await userEvent.click(doc!);
    await expect(args.onTab).toHaveBeenCalledWith("security");
  },
};

export const DataDocLinksToTheRepository: Story = {
  args: { tab: "data" },
  play: async ({ canvas, args }) => {
    // A file of the repository, named by a path relative to the doc, opens on GitHub.
    const [schema] = canvas.getAllByRole("link", { name: "export-v1.schema.json" });
    await userEvent.click(schema!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://github.com/MarJC5/vigie/blob/main/addon/export/export-v1.schema.json");
  },
};

export const AddonsDocInFrench: Story = {
  args: { tab: "addons" },
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    // The table of the known add-ons: its short column keeps its words whole.
    const [table] = canvas.getAllByRole("table");
    await expect(table).toHaveTextContent("FileZilla");
    const [kind] = canvas.getAllByRole("columnheader", { name: "Type" });
    await expect(kind!.getBoundingClientRect().height).toBeLessThan(40);
  },
};
