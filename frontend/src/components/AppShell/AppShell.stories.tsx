import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { hostRows, outdatedSite, sharedHost } from "../../fixtures/scans";
import { sampleTree } from "../../fixtures/tree";
import { AppShell, type EditApi } from "./AppShell";

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

// A fake of the editing services: every edit is accepted, except a rename to "Clients".
function fakeEdit(): EditApi {
  return {
    CreateFolder: fn(async () => ({ id: "new", problem: null })),
    Rename: fn(async (_id: string, name: string) =>
      name === "Clients" ? { code: "problem.duplicateName", detail: "" } : null,
    ),
    Move: fn(async () => null),
    SetColor: fn(async () => null),
    Delete: fn(async () => null),
    Host: fn(async () => ({ name: "Managed VPS", alias: "", hostname: "vps.example.test", port: 2222, user: "deploy", keyPath: "", wpCli: "wp", folderId: "personal", maxConcurrent: 0 })),
    EditHost: fn(async () => ({ keyCleared: false, problem: null })),
    HostKeys: fn(async () => ({ name: "", keys: [], problem: null })),
    Trust: fn(async () => null),
    KeepKey: fn(async () => null),
    DiscoverHost: fn(async () => ({ installs: [], truncated: false, problem: null })),
    AddSites: fn(async () => null),
  };
}

// The first of the two themes: a menu or a dialog opens in the window it came from.
const firstWindow = (canvas: { getAllByRole: (role: string, options?: object) => HTMLElement[] }) =>
  canvas.getAllByRole("complementary", { name: "Site manager" })[0]!.parentElement!;

// A right click opens the menu of a host; "Move to..." opens its dialog, the rest of the window
// out of reach meanwhile.
export const MoveFromTheMenu: Story = {
  args: { edit: fakeEdit(), onChanged: fn() },
  play: async ({ canvas, args }) => {
    const shell = within(firstWindow(canvas));
    await userEvent.pointer({ keys: "[MouseRight]", target: shell.getByRole("treeitem", { name: /Managed VPS/ }) });
    const menu = shell.getByRole("menu", { name: "Actions on Managed VPS" });
    await expect(within(menu).getAllByRole("menuitem").map((i) => i.textContent)).toEqual([
      "Scan⌘R", "Rename↩", "Move to…", "Edit the connection…", "Find new sites…", "Delete the host…⌘⌫",
    ]);
    await userEvent.click(within(menu).getByRole("menuitem", { name: "Move to…" }));
    const dialog = shell.getByRole("dialog", { name: 'Move "Managed VPS"' });
    await expect(firstWindow(canvas).querySelector("nav")).toHaveAttribute("inert");
    await userEvent.click(within(dialog).getByRole("radio", { name: /Clients/ }));
    await userEvent.click(within(dialog).getByRole("button", { name: "Move" }));
    await waitFor(() => expect(args.edit!.Move).toHaveBeenCalledWith("vps", "clients"));
    await waitFor(() => expect(args.onChanged).toHaveBeenCalled());
    await expect(shell.queryByRole("dialog")).toBeNull();
  },
};

// File > New Folder: a folder in the selected folder, named in place.
export const NewFolderFromTheMenuBar: Story = {
  args: {
    edit: fakeEdit(),
    onChanged: fn(),
    initialSelectedId: "shared",
    commands: (handler: (action: string) => void) => {
      setTimeout(() => handler("new-folder"), 0);
      return () => {};
    },
  },
  play: async ({ args }) => {
    await waitFor(() => expect(args.edit!.CreateFolder).toHaveBeenCalledWith("clients", "New folder"));
  },
};

// A refused rename says why in the pane's footer.
export const RenameRefused: Story = {
  args: { edit: fakeEdit(), onChanged: fn(), initialSelectedId: "personal" },
  play: async ({ canvas, args }) => {
    const shell = within(firstWindow(canvas));
    shell.getByRole("treeitem", { name: /Personal/ }).focus();
    await userEvent.keyboard("{Enter}");
    const field = shell.getByRole("textbox", { name: "Name of Personal" });
    await userEvent.clear(field);
    await userEvent.type(field, "Clients{Enter}");
    await waitFor(() => expect(shell.getByRole("alert")).toHaveTextContent("This name is already used here."));
    await expect(args.edit!.Rename).toHaveBeenCalledWith("personal", "Clients");
  },
};

// The "..." next to the title opens the same menu; deleting a host asks first.
export const DeleteFromTheTitleMenu: Story = {
  args: { edit: fakeEdit(), onChanged: fn(), initialSelectedId: "shared", data: { site: async () => outdatedSite, host: async () => sharedHost, sites: async () => hostRows } },
  play: async ({ canvas, args }) => {
    const shell = within(firstWindow(canvas));
    await userEvent.click(await shell.findByRole("button", { name: "More actions" }));
    await userEvent.click(shell.getByRole("menuitem", { name: "Delete the host…" }));
    const dialog = shell.getByRole("dialog", { name: 'Delete "Shared hosting"?' });
    await userEvent.click(within(dialog).getByRole("button", { name: "Delete the host" }));
    await waitFor(() => expect(args.edit!.Delete).toHaveBeenCalledWith("shared"));
  },
};
