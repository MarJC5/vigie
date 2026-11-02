import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { BackupsLeftDialog } from "./BackupsLeftDialog";

const command = "rm -rf ~/.vigie/backups/7c1e4a52-0d55-4f0b-9a8e-3b8c1d2f6a10 ~/.vigie/backups/01a19904-97cc-7220-ba9a-cc46fa0b2922";

const meta = {
  component: BackupsLeftDialog,
  args: { name: "Shared hosting", kind: "host", command, onCopy: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 360 }}><Story /></div>],
} satisfies Meta<typeof BackupsLeftDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// The host is gone from Vigie; its server did not answer: the command to run there, copied.
export const ServerUnreachable: Story = {
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "“Shared hosting” is deleted from Vigie" }).find((d) => d.contains(document.activeElement))!;
    await expect(within(dialog).getByText(command)).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Copy" }));
    await expect(args.onCopy).toHaveBeenCalledWith(command);
    await expect(within(dialog).getByRole("button", { name: "Copied" })).toBeInTheDocument();
    await userEvent.click(within(dialog).getByRole("button", { name: "Close" }));
    await expect(args.onClose).toHaveBeenCalled();
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("dialog", { name: /^«\sShared hosting\s» est supprimé de Vigie$/ })).toHaveLength(2);
  },
};
