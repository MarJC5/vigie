import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { complete } from "../../fixtures/copies";
import { RemoveCopyDialog } from "./RemoveCopyDialog";

const meta = {
  component: RemoveCopyDialog,
  args: { copy: complete, onRemove: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 340 }}><Story /></div>],
} satisfies Meta<typeof RemoveCopyDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// Mockup 28i: the copy goes to the Trash; the focus starts on Cancel.
export const ToTheTrash: Story = {
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Remove this copy?" }).find((d) => d.contains(document.activeElement))!;
    await expect(dialog).toHaveAccessibleDescription("It goes to the Trash.");
    await expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await expect(within(dialog).getByRole("listitem")).toHaveTextContent("2.18 GB");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove" }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};

// No trash takes it: asked again, to delete it for good.
export const ForGood: Story = {
  args: { forGood: true },
  play: async ({ canvas }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Delete this copy for good?" })[0]!;
    await expect(within(dialog).getByRole("button", { name: "Delete for good" })).toBeInTheDocument();
  },
};
