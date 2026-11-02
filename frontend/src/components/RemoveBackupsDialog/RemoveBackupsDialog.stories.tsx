import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { backups } from "../../fixtures/backups";
import { RemoveBackupsDialog } from "./RemoveBackupsDialog";

const meta = {
  component: RemoveBackupsDialog,
  args: { name: "boulangerie-exemple.ch", backups: backups.items!.filter((b) => b.expired), onRemove: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 380 }}><Story /></div>],
} satisfies Meta<typeof RemoveBackupsDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// The backups past the limits, listed with their sizes; the focus starts on Cancel.
export const OneOld: Story = {
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Remove 1 backup of boulangerie-exemple.ch?" }).find((d) => d.contains(document.activeElement))!;
    await expect(dialog).toHaveAccessibleDescription("Vigie removes them on the server, in ~/.vigie/backups. The site's scans and update history stay.");
    await expect(within(dialog).getByRole("button", { name: "Cancel" })).toHaveFocus();
    await expect(within(dialog).getByRole("listitem")).toHaveTextContent("45.1 MB");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove the backup" }));
    await expect(args.onRemove).toHaveBeenCalled();
  },
};
