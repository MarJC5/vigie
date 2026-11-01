import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sampleTags, sampleTree } from "../../fixtures/tree";
import { RenameTagDialog } from "./RenameTagDialog";

const meta = {
  component: RenameTagDialog,
  args: { tag: "Client Dupont", tags: sampleTags, nodes: sampleTree, onRename: fn(async (): Promise<Problem | null> => null), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 460 }}><Story /></div>],
} satisfies Meta<typeof RenameTagDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

const dialog = (canvas: { getAllByRole: (role: string, o: object) => HTMLElement[] }) =>
  canvas.getAllByRole("dialog", { name: "Rename “Client Dupont”" }).find((d) => d.contains(document.activeElement))!;

// A new name: the tag takes it on its sites; unchanged, nothing to confirm.
export const Renamed: Story = {
  play: async ({ canvas, args }) => {
    const d = within(dialog(canvas));
    await expect(dialog(canvas)).toHaveAccessibleDescription("The new name replaces it on its 2 sites.");
    await expect(d.getByRole("button", { name: "Rename" })).toBeDisabled();
    const field = d.getByRole("textbox", { name: "Name" });
    await userEvent.clear(field);
    await userEvent.type(field, "Dupont SA");
    await userEvent.click(d.getByRole("button", { name: "Rename" }));
    await expect(args.onRename).toHaveBeenCalledWith("Dupont SA");
  },
};

// The name of another tag: the dialog says both become one, on how many sites.
export const Merged: Story = {
  play: async ({ canvas }) => {
    const d = within(dialog(canvas));
    const field = d.getByRole("textbox", { name: "Name" });
    await userEvent.clear(field);
    await userEvent.type(field, "prioritaire");
    const note = d.getByRole("note");
    await expect(note).toHaveTextContent("“Prioritaire” already exists");
    await expect(note).toHaveTextContent("Both become one tag, on 2 sites.");
  },
};

// A name the Go side refuses: the dialog stays and says why.
export const Refused: Story = {
  args: { onRename: fn(async (): Promise<Problem | null> => ({ code: "problem.tagCharacter", detail: "" })) },
  play: async ({ canvas }) => {
    const d = within(dialog(canvas));
    const field = d.getByRole("textbox", { name: "Name" });
    await userEvent.type(field, ", SA");
    await userEvent.click(d.getByRole("button", { name: "Rename" }));
    await waitFor(() => expect(field).toHaveAttribute("aria-invalid", "true"));
    await expect(d.getByText("A tag cannot contain a comma or a vertical bar (|).")).toBeVisible();
  },
};
