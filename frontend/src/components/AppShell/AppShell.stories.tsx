import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { hostRows, outdatedSite, sharedHost } from "../../fixtures/scans";
import { sampleTree } from "../../fixtures/tree";
import { AppShell } from "./AppShell";

// Each story renders once per theme, so the window landmarks repeat by construction;
// the end-to-end test checks that the window has one of each.
const twoThemes = {
  a11y: {
    config: {
      rules: [
        { id: "landmark-unique", enabled: false },
        { id: "landmark-no-duplicate-main", enabled: false },
      ],
    },
  },
};

const meta = {
  component: AppShell,
  args: { nodes: sampleTree },
  parameters: { layout: "fullscreen", ...twoThemes },
  decorators: [
    (Story) => (
      <div style={{ height: 760 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AppShell>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithSites: Story = {
  play: async ({ canvas }) => {
    const [item] = canvas.getAllByRole("treeitem", {
      name: /reseau-ecoles.ch/,
    });
    await userEvent.click(item!);
    const [heading] = canvas.getAllByRole("heading", {
      level: 1,
      name: "reseau-ecoles.ch",
    });
    await expect(heading).toBeInTheDocument();
    await expect(canvas.getAllByText("Multisite network").length).toBeGreaterThan(0);
  },
};

export const FirstLaunch: Story = {
  args: { nodes: [], onAddHost: fn() },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Add your first host" })).toHaveLength(2);
    // The button of the empty state and the "+" of the rail both open the wizard.
    const [add, plus] = [
      canvas.getAllByRole("button", { name: "Add a host" }).find((b) => b.textContent),
      canvas.getAllByRole("button", { name: "Add a host" }).find((b) => !b.textContent),
    ];
    await userEvent.click(add!);
    await userEvent.click(plus!);
    await expect(args.onAddHost).toHaveBeenCalledTimes(2);
  },
};

export const HostJustAdded: Story = {
  args: { initialSelectedId: "shared" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Shared hosting" })).toHaveLength(2);
  },
};

// The data of the window, as the Go side serves it.
const data = {
  site: async () => outdatedSite,
  host: async () => sharedHost,
  sites: async () => hostRows,
};

export const SiteWithItsScan: Story = {
  args: { data, initialSelectedId: "boulangerie", onScan: fn() },
  play: async ({ canvas, args }) => {
    await expect((await canvas.findAllByRole("heading", { level: 1, name: "boulangerie-exemple.ch" }))[0]).toBeInTheDocument();
    await expect(canvas.getAllByRole("tabpanel")[0]).toHaveTextContent("7.1.3 · 7.1.4 available");
    await userEvent.click(canvas.getAllByRole("button", { name: "Scan now" })[0]!);
    await expect(args.onScan).toHaveBeenCalledWith("boulangerie");
  },
};

export const HostWithItsSites: Story = {
  args: { data, initialSelectedId: "shared", onScan: fn() },
  play: async ({ canvas }) => {
    await expect((await canvas.findAllByText("u12ab@ssh.example-host.ch · port 22 · wp-cli"))[0]).toBeInTheDocument();
    await expect(canvas.getAllByRole("table", { name: "Sites of Shared hosting" })).toHaveLength(2);
  },
};

export const ScansRunning: Story = {
  args: {
    data,
    initialSelectedId: "shared",
    scans: { running: [{ nodeId: "shared", sites: 3, done: 1 }], completed: 0 },
    onStopAll: fn(),
  },
  play: async ({ canvas, args }) => {
    const [pane] = canvas.getAllByRole("complementary", { name: "Site manager" });
    await expect(pane).toHaveTextContent("2 scans running");
    await userEvent.click(within(pane!).getByRole("button", { name: "Stop all" }));
    await expect(args.onStopAll).toHaveBeenCalled();
    // The host being scanned says so, in its button.
    await expect((await canvas.findAllByRole("button", { name: "Scanning…" }))[0]).toHaveAttribute("aria-busy", "true");
  },
};

export const StatusDotsInTheTree: Story = {
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree");
    await expect(within(tree!).getAllByRole("img", { name: "Needs attention" })).toHaveLength(1);
    await expect(within(tree!).getAllByRole("img", { name: "Up to date" })).toHaveLength(1);
  },
};

// The menu bar's commands, as the Go side sends them.
function commands() {
  let handler: ((action: string) => void) | undefined;
  return {
    subscribe: (h: (action: string) => void) => {
      handler = h;
      return () => {
        handler = undefined;
      };
    },
    send: (action: string) => handler?.(action),
    subscribed: () => handler !== undefined,
  };
}

const menu = commands();

export const MenuCommands: Story = {
  args: { data, initialSelectedId: "shared", onScan: fn(), commands: menu.subscribe },
  play: async ({ canvas, args }) => {
    // Rendered twice (one per theme): the last subscriber is the second render. The menu only
    // reaches a window that has subscribed.
    await waitFor(() => expect(menu.subscribed()).toBe(true));
    menu.send("search");
    const fields = await canvas.findAllByRole("searchbox", { name: "Search" });
    await waitFor(() => expect(fields.at(-1)).toHaveFocus());
    menu.send("scan");
    await expect(args.onScan).toHaveBeenCalledWith("shared");
  },
};
