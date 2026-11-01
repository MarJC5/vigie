import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { maxNotes, NotesPanel } from "./NotesPanel";

const notes = `**Client**: Boulangerie Exemple, Marie Exemple.

### Contract

- Monthly maintenance, until 30 June 2027
- WooCommerce: tell Marie before any major update

Signed quote: [shared folder](https://exemple.ch/devis)`;

// Each story renders once per theme, so the notes' region repeats by construction.
const twoThemes = { a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } } };

const meta = {
  component: NotesPanel,
  args: { notes, name: "boulangerie-exemple.ch", onSave: fn(async (): Promise<Problem | null> => null), onOpen: fn() },
  parameters: twoThemes,
} satisfies Meta<typeof NotesPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

// Read as a document; a link opens in the browser.
export const Read: Story = {
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { name: "Contract" })).toHaveLength(2);
    const [link] = canvas.getAllByRole("link", { name: "shared folder" });
    await userEvent.click(link!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://exemple.ch/devis");
  },
};

// Edited in place, then saved.
export const Edited: Story = {
  play: async ({ canvas, args }) => {
    const [edit] = canvas.getAllByRole("button", { name: "Edit" });
    await userEvent.click(edit!);
    const [field] = canvas.getAllByRole("textbox", { name: "Notes of boulangerie-exemple.ch" });
    await expect(field).toHaveValue(notes);
    await userEvent.type(field!, "\nCall first.");
    await expect(canvas.getAllByText(`${notes.length + 12} / 20,000`).length).toBeGreaterThan(0);
    const [save] = canvas.getAllByRole("button", { name: "Save" });
    await userEvent.click(save!);
    await expect(args.onSave).toHaveBeenCalledWith(`${notes}\nCall first.`);
    await waitFor(() => expect(canvas.queryAllByRole("textbox")).toHaveLength(0));
  },
};

// No notes yet; HTML written in them is shown as text.
export const Empty: Story = {
  args: { notes: "" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("No notes yet.")).toHaveLength(2);
    await expect(canvas.getAllByRole("button", { name: "Add notes" })).toHaveLength(2);
  },
};

export const RawHtmlIsText: Story = {
  args: { notes: "Some <em>emphasis</em> written in HTML." },
  play: async ({ canvas, canvasElement }) => {
    await expect(canvas.getAllByText(/<em>emphasis<\/em>/).length).toBeGreaterThan(0);
    await expect(canvasElement.querySelector("em")).toBeNull();
  },
};

// Too long: the count says so, Save waits; a refusal of the Go side keeps the notes in edition.
export const TooLong: Story = {
  args: { notes: "x".repeat(maxNotes + 1) },
  play: async ({ canvas }) => {
    const [edit] = canvas.getAllByRole("button", { name: "Edit" });
    await userEvent.click(edit!);
    const [save] = canvas.getAllByRole("button", { name: "Save" });
    await expect(save).toBeDisabled();
    await expect(canvas.getAllByText("20,001 / 20,000").length).toBeGreaterThan(0);
  },
};

export const Refused: Story = {
  args: { onSave: fn(async (): Promise<Problem | null> => ({ code: "problem.notesTooLong", detail: "" })) },
  play: async ({ canvas }) => {
    const [edit] = canvas.getAllByRole("button", { name: "Edit" });
    await userEvent.click(edit!);
    const [save] = canvas.getAllByRole("button", { name: "Save" });
    await userEvent.click(save!);
    await waitFor(() => expect(canvas.getAllByText("Notes hold 20,000 characters at most.").length).toBeGreaterThan(0));
    await expect(canvas.getAllByRole("textbox").length).toBeGreaterThan(0);
  },
};
