import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fireEvent, fn, userEvent, waitFor, within } from "storybook/test";

import { sampleTree } from "../../fixtures/tree";
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
