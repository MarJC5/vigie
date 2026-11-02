import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { findings, xss } from "../../fixtures/findings";
import { hostRows, outdatedSite, sharedHost } from "../../fixtures/scans";
import { sampleTree } from "../../fixtures/tree";
import { completed, preview, running } from "../../fixtures/updates";
import type { UpdatesApi } from "../UpdateFlow/UpdateFlow";
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

// The window opens on the first host, never on an empty page.
export const OpensOnTheFirstHost: Story = {
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Shared hosting" })).toHaveLength(2);
    const [host] = canvas.getAllByRole("treeitem", { name: /^Shared hosting/ });
    await expect(host).toHaveAttribute("aria-selected", "true");
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

// The next automatic scan at the foot of the site list: its time today, its day otherwise.
const today = new Date();
today.setHours(18, 0, 0, 0);
export const NextAutomaticScan: Story = {
  args: { data, initialSelectedId: "shared", nextScan: today.toISOString() },
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    // Quiet: the time shown, the sentence read and on hover.
    await expect((await canvas.findAllByText("Prochain relevé : 18:00"))[0]).toBeInTheDocument();
    await expect(canvas.getAllByTitle("Prochain relevé : 18:00")[0]).toHaveTextContent(/^18:00/);
  },
};

const updatesApi = (): UpdatesApi => ({
  Preview: fn(async () => [preview]),
  Update: fn(async () => null),
  UpdateAll: fn(async () => null),
  Rollback: fn(async () => null),
  CancelAll: fn(async () => {}),
  History: fn(async () => [completed]),
});

// The data of a site whose id is the tree's, as in the window.
const boulangerie = { ...data, site: async () => ({ ...outdatedSite, id: "boulangerie" }) };

// A site in an update: counted in the footer, not scanned, and its window opens again from its page.
export const SiteUpdating: Story = {
  args: {
    data: { ...boulangerie, history: async () => [running, completed] },
    initialSelectedId: "boulangerie",
    onScan: fn(),
    updates: { running: [{ ...running, siteId: "boulangerie" }], active: 1, bulk: [], completed: 0, sites: ["boulangerie"] },
    updatesApi: updatesApi(),
  },
  play: async ({ canvas, args }) => {
    const [pane] = canvas.getAllByRole("complementary", { name: "Site manager" });
    await expect(pane).toHaveTextContent("1 site updating");
    await userEvent.click(within(pane!).getByRole("button", { name: "Stop all" }));
    await expect(args.updatesApi!.CancelAll).toHaveBeenCalled();
    await userEvent.click((await canvas.findAllByRole("button", { name: "Follow the update" }))[0]!);
    await expect((await canvas.findAllByRole("heading", { level: 1, name: "Updating boulangerie-exemple.ch" }))[0]).toBeInTheDocument();
    // The window closes back on the site.
    await userEvent.click(canvas.getAllByRole("button", { name: "Close" })[0]!);
    await expect((await canvas.findAllByRole("heading", { level: 1, name: "boulangerie-exemple.ch" }))[0]).toBeInTheDocument();
    await expect(args.onScan).not.toHaveBeenCalled();
  },
};

export const UpdateFromTheTab: Story = {
  args: { data: { ...boulangerie, history: async () => [completed] }, initialSelectedId: "boulangerie", updatesApi: updatesApi() },
  play: async ({ canvas, args }) => {
    await userEvent.click((await canvas.findAllByRole("tab", { name: /Updates/ }))[0]!);
    await userEvent.click(canvas.getAllByRole("button", { name: "Update (4)" })[0]!);
    await expect((await canvas.findAllByRole("heading", { level: 1, name: "Update boulangerie-exemple.ch" }))[0]).toBeInTheDocument();
    await expect(args.updatesApi!.Preview).toHaveBeenCalledWith("boulangerie");
  },
};

export const UpdateAllOfAHost: Story = {
  args: { data, initialSelectedId: "shared", updatesApi: updatesApi() },
  play: async ({ canvas, args }) => {
    await userEvent.click((await canvas.findAllByRole("button", { name: "Update all" }))[0]!);
    await expect(args.updatesApi!.Preview).toHaveBeenCalledWith("shared");
    await userEvent.click((await canvas.findAllByRole("button", { name: "Update 1 site" }))[0]!);
    await waitFor(() => expect(args.updatesApi!.UpdateAll).toHaveBeenCalledWith("shared"));
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
    Delete: fn(async () => ({ problem: null, left: [], command: "" })),
    Backups: fn(async () => []),
    SetAdminURL: fn(async () => null),
    SetNotes: fn(async () => null),
    SetTags: fn(async () => null),
    RenameTag: fn(async () => null),
    RemoveTag: fn(async () => null),
    TestConnection: fn(async () => ({ phpVersion: "8.4.27", wpCli: "wp", wpCliVersion: "2.12.0", problem: null })),
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
      "Scan⌘R", "New folder⇧⌘N", "Rename↩", "Move to…", "Edit the connection…", "Test the connection…", "Find new sites…", "Delete the host…⌘⌫",
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
    await waitFor(() => expect(args.edit!.Delete).toHaveBeenCalledWith("shared", true));
  },
};

// The empty space of the site list has its own menu: a folder at the top level, a host.
export const MenuOfTheEmptySpace: Story = {
  args: { edit: fakeEdit(), onChanged: fn(), onAddHost: fn() },
  play: async ({ canvas, args }) => {
    const [pane] = canvas.getAllByRole("complementary", { name: "Site manager" });
    const empty = pane!.querySelector<HTMLElement>("[data-pane-body]")!;
    const box = empty.getBoundingClientRect();
    empty.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: box.left + 20, clientY: box.bottom - 20 }));
    const [menu] = await canvas.findAllByRole("menu", { name: "Site manager" });
    await expect(within(menu!).getByRole("menuitem", { name: /Add a host/ })).toBeInTheDocument();
    await userEvent.click(within(menu!).getByRole("menuitem", { name: /New folder/ }));
    await waitFor(() => expect(args.edit!.CreateFolder).toHaveBeenCalledWith("", "New folder"));
  },
};

// A host's menu also makes a folder, next to the host.
export const NewFolderNextToAHost: Story = {
  args: { edit: fakeEdit(), onChanged: fn(), initialSelectedId: "shared" },
  play: async ({ canvas, args }) => {
    const [host] = canvas.getAllByRole("treeitem", { name: /^Shared hosting/ });
    host!.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 40, clientY: 40 }));
    const [menu] = await canvas.findAllByRole("menu", { name: "Actions on Shared hosting" });
    await userEvent.click(within(menu!).getByRole("menuitem", { name: /New folder/ }));
    await waitFor(() => expect(args.edit!.CreateFolder).toHaveBeenCalledWith("clients", "New folder"));
  },
};

// A row of a host's table opens the menu of its site, and the page stays on the host.
export const MenuOfATableRow: Story = {
  args: { data, edit: fakeEdit(), onChanged: fn(), initialSelectedId: "shared" },
  play: async ({ canvas }) => {
    const [table] = await canvas.findAllByRole("table", { name: "Sites of Shared hosting" });
    // atelier-exemple.ch has the same id in the table and in the tree, as in the window.
    const row = within(table!).getByRole("button", { name: "atelier-exemple.ch" }).closest("tr")!;
    row.dispatchEvent(new MouseEvent("contextmenu", { bubbles: true, cancelable: true, clientX: 600, clientY: 300 }));
    const [menu] = await canvas.findAllByRole("menu", { name: "Actions on atelier-exemple.ch" });
    await expect(within(menu!).getByRole("menuitem", { name: /Rename/ })).toBeInTheDocument();
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Shared hosting" })[0]).toBeInTheDocument();
  },
};

// A finding set aside from the Security tab: the dialog, the call to the Go side, the detail read
// again and the tree's statuses too.
export const IgnoreAFinding: Story = {
  args: {
    data: {
      ...data,
      site: async () => ({
        ...outdatedSite,
        id: "boulangerie",
        scan: { ...outdatedSite.scan!, status: "danger", findings, counts: { urgent: 2, attention: 5, notChecked: 1, ignored: 1 } },
      }),
    },
    initialSelectedId: "boulangerie",
    security: { Ignore: fn(async () => null), Unignore: fn(async () => null) },
    onChanged: fn(),
    onCheckFiles: fn(),
  },
  play: async ({ canvasElement, args }) => {
    const window = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.click(await window.findByRole("tab", { name: /Security/ }));
    await userEvent.click(window.getByRole("button", { name: /Ignore “Contact Form 7/ }));
    const dialog = within(window.getByRole("dialog"));
    await userEvent.click(dialog.getByRole("button", { name: "Ignore" }));
    await expect(args.security!.Ignore).toHaveBeenCalledWith("boulangerie", xss.key, "6.0.4", "");
    await waitFor(() => expect(window.queryByRole("dialog")).toBeNull());
    await expect(args.onChanged).toHaveBeenCalled();
    await userEvent.click(window.getByRole("button", { name: "Check files now" }));
    await expect(args.onCheckFiles).toHaveBeenCalledWith("boulangerie");
  },
};

// Help > Vigie Help in the menu bar opens the Help page.
export const HelpFromTheMenuBar: Story = {
  args: {
    onSection: fn(),
    commands: (handler: (action: string) => void) => {
      setTimeout(() => handler("help"), 0);
      return () => {};
    },
  },
  play: async ({ args }) => {
    await waitFor(() => expect(args.onSection).toHaveBeenCalledWith("help"));
  },
};

// The Help page takes the place of the site list, with Help current in the rail.
export const Help: Story = {
  args: { section: "help", onSection: fn(), helpPage: <h1>Help</h1> },
  play: async ({ canvas }) => {
    await expect(canvas.queryByRole("tree")).toBeNull();
    const [help] = canvas.getAllByRole("link", { name: "Help" });
    await expect(help).toHaveAttribute("aria-current", "page");
  },
};

// The rail opens the settings, which take the place of the site list.
export const Settings: Story = {
  args: { section: "settings", onSection: fn(), settingsPage: <h1>Settings</h1> },
  play: async ({ canvas, args }) => {
    await expect(canvas.queryByRole("tree")).toBeNull();
    const [settings] = canvas.getAllByRole("link", { name: "Settings" });
    await expect(settings).toHaveAttribute("aria-current", "page");
    const [sites] = canvas.getAllByRole("link", { name: "Sites" });
    await userEvent.click(sites!);
    await expect(args.onSection).toHaveBeenCalledWith("sites");
  },
};
