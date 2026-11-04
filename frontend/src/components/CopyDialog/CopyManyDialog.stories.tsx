import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { CopyManyDialog } from "./CopyDialog";

const meta = {
  component: CopyManyDialog,
  args: { count: 4, onCopy: fn(async (): Promise<Problem | null> => null), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 480 }}><Story /></div>],
} satisfies Meta<typeof CopyManyDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// Mockup 28k: the content only, the database and wp-content by default; the whole site chosen.
export const Selection: Story = {
  play: async ({ canvas, args }) => {
    const dialog = within(canvas.getAllByRole("dialog", { name: "Copy 4 sites to this computer" })[0]!);
    await expect(dialog.getByRole("button", { name: /^The database and wp-content/ })).toHaveAttribute("aria-pressed", "true");
    await expect(dialog.getByText(/^One copy per site, in the folder of the copies\./)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: /^The whole site/ }));
    await userEvent.click(dialog.getByRole("button", { name: "Copy the 4 sites" }));
    await expect(args.onCopy).toHaveBeenCalledWith("site");
    await expect(args.onClose).toHaveBeenCalled();
  },
};

// A copy of one of the sites runs already: refused, said in the dialog.
export const Refused: Story = {
  args: { onCopy: fn(async (): Promise<Problem | null> => ({ code: "problem.copyRunning", detail: "" })) },
  play: async ({ canvas, args }) => {
    const dialog = within(canvas.getAllByRole("dialog", { name: "Copy 4 sites to this computer" })[0]!);
    await userEvent.click(dialog.getByRole("button", { name: "Copy the 4 sites" }));
    await expect(await dialog.findByRole("alert")).toHaveTextContent("A copy of this site is running.");
    await expect(args.onClose).not.toHaveBeenCalled();
  },
};
