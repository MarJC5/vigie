import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sampleTags } from "../../fixtures/tree";
import { TagRow } from "./TagRow";

// Each story renders once per theme, so the lists repeat by construction.
const twoThemes = { a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }] } } };

const meta = {
  component: TagRow,
  args: {
    tags: ["Contrat de maintenance", "Client Dupont"],
    used: sampleTags,
    onSave: fn(async (): Promise<Problem | null> => null),
    onOpenTag: fn(),
  },
  parameters: twoThemes,
} satisfies Meta<typeof TagRow>;
export default meta;
type Story = StoryObj<typeof meta>;

// A tag opens its page; its button takes it off the site.
export const Tags: Story = {
  play: async ({ canvas, args }) => {
    const [tag] = canvas.getAllByRole("button", { name: "Client Dupont" });
    await userEvent.click(tag!);
    await expect(args.onOpenTag).toHaveBeenCalledWith("Client Dupont");
    const [remove] = canvas.getAllByRole("button", { name: "Remove the tag Contrat de maintenance" });
    await userEvent.click(remove!);
    await expect(args.onSave).toHaveBeenCalledWith(["Client Dupont"]);
  },
};

// Typed: the tags in use that hold it, then a new one; the arrows choose, Enter adds.
export const Adding: Story = {
  play: async ({ canvas, args }) => {
    const [add] = canvas.getAllByRole("button", { name: "Add a tag" });
    await userEvent.click(add!);
    const field = canvas.getByRole("combobox", { name: "New tag" });
    await userEvent.type(field, "pri");
    const options = canvas.getAllByRole("option");
    await expect(options.map((o) => o.textContent)).toEqual(["Prioritaire2 sites", "Priorité basse1 site", "Create “pri”new"]);
    await expect(options[0]).toHaveAttribute("aria-selected", "true");
    await userEvent.keyboard("{ArrowDown}{Enter}");
    await expect(args.onSave).toHaveBeenCalledWith(["Contrat de maintenance", "Client Dupont", "Priorité basse"]);
    // The field stays open, empty, for another one.
    await waitFor(() => expect(field).toHaveValue(""));
  },
};

// Escape closes the field and gives the focus back to "Add a tag".
export const Cancelled: Story = {
  play: async ({ canvas, args }) => {
    const [add] = canvas.getAllByRole("button", { name: "Add a tag" });
    await userEvent.click(add!);
    await userEvent.keyboard("Contract{Escape}");
    await expect(canvas.queryByRole("combobox")).toBeNull();
    await expect(canvas.getAllByRole("button", { name: "Add a tag" })[0]).toHaveFocus();
    await expect(args.onSave).not.toHaveBeenCalled();
  },
};

// A tag the Go side refuses: the field stays, and says why.
export const Refused: Story = {
  args: { onSave: fn(async (): Promise<Problem | null> => ({ code: "problem.tagCharacter", detail: "" })) },
  play: async ({ canvas }) => {
    const [add] = canvas.getAllByRole("button", { name: "Add a tag" });
    await userEvent.click(add!);
    await userEvent.keyboard("a,b{ArrowDown}{Enter}");
    await waitFor(() => expect(canvas.getByRole("combobox")).toHaveAttribute("aria-invalid", "true"));
    await expect(canvas.getByText("A tag cannot contain a comma or a vertical bar (|).")).toBeVisible();
  },
};

// Without a way to save, the tags are only shown; with none, nothing is.
export const ReadOnly: Story = {
  args: { onSave: undefined, onOpenTag: undefined },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Client Dupont")).toHaveLength(2);
    await expect(canvas.queryAllByRole("button")).toHaveLength(0);
  },
};
