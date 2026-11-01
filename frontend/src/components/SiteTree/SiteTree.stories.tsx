import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fireEvent, fn, userEvent, waitFor, within } from "storybook/test";

import { networkTree } from "../../fixtures/subsites";
import { withSubsites } from "../../lib/subsites";
import { sampleTags, sampleTree } from "../../fixtures/tree";
import { SiteTree } from "./SiteTree";

function Controlled() {
  const [selected, setSelected] = useState("atelier");
  return <SiteTree label="Site manager" nodes={sampleTree} selectedId={selected} onSelect={setSelected} />;
}

const meta = {
  component: SiteTree,
  args: {
    label: "Site manager",
    nodes: sampleTree,
    selectedId: "atelier",
    onSelect: () => {},
  },
} satisfies Meta<typeof SiteTree>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Default: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree", { name: "Site manager" });
    await expect(tree).toBeInTheDocument();
    // A multisite network shows its subsite count.
    await expect(canvas.getAllByText("14 subsites")).toHaveLength(2);
  },
};

export const Keyboard: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [atelier] = canvas.getAllByRole("treeitem", {
      name: /atelier-exemple.ch/,
    });
    atelier!.focus();
    await userEvent.keyboard("{ArrowDown}");
    const [boulangerie] = canvas.getAllByRole("treeitem", {
      name: /boulangerie-exemple.ch/,
    });
    await expect(boulangerie).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await expect(boulangerie).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowUp}{ArrowUp}");
    const [shared] = canvas.getAllByRole("treeitem", {
      name: /Shared hosting/,
    });
    await expect(shared).toHaveFocus();
  },
};

export const Collapse: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [clients] = canvas.getAllByRole("treeitem", { name: /Clients/ });
    await userEvent.click(clients!.querySelector("[data-caret]")!);
    await expect(clients).toHaveAttribute("aria-expanded", "false");
    await expect(canvas.queryAllByRole("treeitem", { name: /atelier-exemple.ch/ })).toHaveLength(1);
  },
};

// A network just added: its subsites are read at its first scan, so no count is shown yet.
export const NetworkNotScannedYet: Story = {
  args: {
    nodes: sampleTree.map((n) => (n.id === "ecoles" ? { ...n, subsites: 0 } : n)),
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryAllByText(/subsites/)).toHaveLength(0);
  },
};

// The search keeps what matches and the folders and hosts that lead to it, accents ignored.
export const Filtered: Story = {
  args: { filter: "ECOLES" },
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree");
    const names = within(tree!).getAllByRole("treeitem").map((i) => i.textContent);
    await expect(names).toHaveLength(3);
    await expect(names[2]).toContain("reseau-ecoles.ch");
  },
};

// The search reads the notes too: a site found by its notes says so (ADR-0023).
export const FoundInTheNotes: Story = {
  args: { filter: "contrat" },
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree");
    const rows = within(tree!).getAllByRole("treeitem");
    await expect(rows).toHaveLength(3);
    await expect(rows[2]).toHaveTextContent("boulangerie-exemple.ch");
    await expect(rows[2]).toHaveTextContent("in the notes");
  },
};

// The search reads the tags too (ADR-0025): the sites found by one say so.
export const FoundInTheTags: Story = {
  args: { filter: "dupont" },
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree");
    const rows = within(tree!).getAllByRole("treeitem");
    await expect(rows).toHaveLength(4);
    await expect(rows[2]).toHaveTextContent("atelier-exemple.chin the tags");
    await expect(rows[3]).toHaveTextContent("boulangerie-exemple.chin the tags");
  },
};

// The row being renamed is a field: Enter keeps the name, Escape cancels.
export const RenameInPlace: Story = {
  args: { editingId: "personal", onRename: fn(), onCancelEdit: fn() },
  play: async ({ canvas, args }) => {
    // The field takes the focus as it appears (the last of the two themes here).
    const field = canvas.getAllByRole("textbox", { name: "Name of Personal" }).find((f) => f === document.activeElement);
    await expect(field).toHaveValue("Personal");
    await userEvent.clear(field!);
    await userEvent.type(field!, "Archives{Enter}");
    await expect(args.onRename).toHaveBeenCalledWith("personal", "Archives");
  },
};

export const CancelRename: Story = {
  args: { editingId: "personal", onRename: fn(), onCancelEdit: fn() },
  play: async ({ canvas, args }) => {
    const field = canvas.getAllByRole("textbox", { name: "Name of Personal" }).find((f) => f === document.activeElement);
    await userEvent.type(field!, "x{Escape}");
    await expect(args.onCancelEdit).toHaveBeenCalled();
    field!.blur();
    await expect(args.onRename).not.toHaveBeenCalled();
  },
};

// As in the Finder: Enter on the selected item renames it, Command-Backspace deletes it.
export const EditFromTheKeyboard: Story = {
  args: { onEdit: fn(), onDelete: fn(), onMenu: fn() },
  play: async ({ canvas, args }) => {
    const [atelier] = canvas.getAllByRole("treeitem", { name: /atelier-exemple.ch/ });
    atelier!.focus();
    await userEvent.keyboard("{Enter}");
    await expect(args.onEdit).toHaveBeenCalledWith("atelier");
    await userEvent.keyboard("{Meta>}{Backspace}{/Meta}");
    await expect(args.onDelete).toHaveBeenCalledWith("atelier");
    await userEvent.keyboard("{Shift>}{F10}{/Shift}");
    await expect(args.onMenu).toHaveBeenCalledWith("atelier", expect.anything());
  },
};

export const ContextMenu: Story = {
  args: { onMenu: fn() },
  play: async ({ canvas, args }) => {
    const [shared] = canvas.getAllByRole("treeitem", { name: /Shared hosting/ });
    await userEvent.pointer({ keys: "[MouseRight]", target: shared! });
    await expect(args.onMenu).toHaveBeenCalledWith("shared", expect.anything());
  },
};

// A host dragged onto a folder goes into it; onto a host or a site, into that row's folder.
export const DragAHost: Story = {
  args: { onMove: fn(), onDragChange: fn() },
  play: async ({ canvas, args }) => {
    const row = (name: RegExp) => canvas.getAllByRole("treeitem", { name })[0]!;
    await expect(row(/atelier-exemple.ch/)).toHaveAttribute("draggable", "false");
    const data = new DataTransfer();
    fireEvent.dragStart(row(/Managed VPS/), { dataTransfer: data });
    fireEvent.dragOver(row(/Clients/), { dataTransfer: data });
    await waitFor(() => expect(row(/Clients/)).toHaveAttribute("data-drop-target", "true"));
    await expect(args.onDragChange).toHaveBeenLastCalledWith({ id: "vps", target: "clients" });
    fireEvent.dragOver(row(/boulangerie-exemple.ch/), { dataTransfer: data });
    await expect(args.onDragChange).toHaveBeenLastCalledWith({ id: "vps", target: "clients" });
    fireEvent.drop(row(/boulangerie-exemple.ch/), { dataTransfer: data });
    await expect(args.onMove).toHaveBeenCalledWith("vps", "clients");
    fireEvent.dragEnd(row(/Managed VPS/), { dataTransfer: data });
    await expect(args.onDragChange).toHaveBeenLastCalledWith(null);
  },
};

// A folder cannot go into itself, and its own place is no move: nothing is offered.
export const NoMoveIntoItself: Story = {
  args: { onMove: fn(), onDragChange: fn() },
  play: async ({ canvas, args }) => {
    const row = (name: RegExp) => canvas.getAllByRole("treeitem", { name })[0]!;
    const data = new DataTransfer();
    fireEvent.dragStart(row(/Clients/), { dataTransfer: data });
    fireEvent.dragOver(row(/Shared hosting/), { dataTransfer: data });
    await expect(args.onDragChange).toHaveBeenLastCalledWith({ id: "clients", target: undefined });
    fireEvent.drop(row(/Shared hosting/), { dataTransfer: data });
    await expect(args.onMove).not.toHaveBeenCalled();
    // A drop ends the drag: the next try starts again.
    fireEvent.dragStart(row(/Clients/), { dataTransfer: data });
    fireEvent.dragOver(row(/Personal/), { dataTransfer: data });
    fireEvent.drop(row(/Personal/), { dataTransfer: data });
    await expect(args.onMove).toHaveBeenCalledWith("clients", "personal");
  },
};

// The empty space under the rows is the root.
export const DropAtTheRoot: Story = {
  args: { onMove: fn(), onDragChange: fn() },
  play: async ({ canvas, args }) => {
    const [vps] = canvas.getAllByRole("treeitem", { name: /Managed VPS/ });
    const [tree] = canvas.getAllByRole("tree");
    const data = new DataTransfer();
    fireEvent.dragStart(vps!, { dataTransfer: data });
    fireEvent.dragOver(tree!, { dataTransfer: data });
    await waitFor(() => expect(tree).toHaveAttribute("data-drop-target", "true"));
    fireEvent.drop(tree!, { dataTransfer: data });
    await expect(args.onMove).toHaveBeenCalledWith("vps", "");
  },
};

// The Tags group under the folders (ADR-0025): a tag opens its page, its menu; the group folds.
export const Tags: Story = {
  args: { tags: sampleTags, selectedId: "tag:Prioritaire", onSelect: fn(), onMenu: fn() },
  play: async ({ canvas, args }) => {
    const [tree] = canvas.getAllByRole("tree");
    const rows = within(tree!);
    const group = rows.getByRole("treeitem", { name: "Tags" });
    await expect(group).toHaveAttribute("aria-expanded", "true");
    const tag = rows.getByRole("treeitem", { name: "Contrat de maintenance 6" });
    await expect(rows.getByRole("treeitem", { name: "Prioritaire 2" })).toHaveAttribute("aria-selected", "true");
    await userEvent.click(tag);
    await expect(args.onSelect).toHaveBeenCalledWith("tag:Contrat de maintenance");
    fireEvent.contextMenu(tag, { clientX: 40, clientY: 300 });
    await expect(args.onMenu).toHaveBeenCalledWith("tag:Contrat de maintenance", { x: 40, y: 300 });
    await userEvent.click(group);
    await expect(group).toHaveAttribute("aria-expanded", "false");
    await expect(rows.queryByRole("treeitem", { name: /^Prioritaire/ })).toBeNull();
  },
};

// The search keeps the tags that hold what is typed.
export const TagsFound: Story = {
  args: { tags: sampleTags, filter: "prio" },
  play: async ({ canvas }) => {
    const [tree] = canvas.getAllByRole("tree");
    const names = within(tree!).getAllByRole("treeitem").map((r) => r.textContent);
    await expect(names).toEqual(["Clients", "Shared hosting5", "boulangerie-exemple.chin the tags", "Tags", "Prioritaire2", "Priorité basse1"]);
  },
};

// A network unfolds to its subsites, folded by default (ADR-0029); a subsite opens its page, and an
// archived one says so.
export const ANetworksSubsites: Story = {
  args: { nodes: withSubsites(networkTree), onSelect: fn(), onCollapsedChange: fn() },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const tree = within(within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!).getByRole("tree"));
    const network = tree.getByRole("treeitem", { name: /reseau-ecoles\.ch/ });
    await expect(network).toHaveAttribute("aria-expanded", "false");
    await expect(tree.queryByRole("treeitem", { name: /^\/alpha/ })).toBeNull();
    await userEvent.click(network.querySelector("[data-caret]")!);
    await expect(args.onCollapsedChange).toHaveBeenCalledWith(new Set(["open:ecoles"]));
  },
};

export const ANetworkUnfolded: Story = {
  args: { nodes: withSubsites(networkTree), collapsed: new Set(["open:ecoles"]), selectedId: "subsite:ecoles:2", onSelect: fn() },
  globals: { lang: "fr" },
  play: async ({ canvasElement, args }) => {
    const tree = within(within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!).getByRole("tree"));
    await expect(tree.getByRole("treeitem", { name: /reseau-ecoles\.ch/ })).toHaveAttribute("aria-expanded", "true");
    await expect(tree.getByRole("treeitem", { name: "/alpha" })).toHaveAttribute("aria-selected", "true");
    await expect(tree.getByRole("treeitem", { name: /^\/gamma/ })).toHaveTextContent("archivé");
    await userEvent.click(tree.getByRole("treeitem", { name: "/beta" }));
    await expect(args.onSelect).toHaveBeenCalledWith("subsite:ecoles:3");
  },
};
