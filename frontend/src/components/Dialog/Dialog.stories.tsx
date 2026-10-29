import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { Button } from "../Button/Button";
import { TextField } from "../TextField/TextField";
import { Dialog } from "./Dialog";

const meta = {
  component: Dialog,
  args: {
    title: 'Delete "Shared hosting"?',
    lead: "Its 5 sites, their scans and their update history are removed from Vigie.",
    onClose: fn(),
    children: <p style={{ margin: 0 }}>Nothing changes on the server.</p>,
    actions: (
      <>
        <Button variant="outline">Cancel</Button>
        <Button variant="danger">Delete the host</Button>
      </>
    ),
  },
  decorators: [(Story) => <div style={{ position: "relative", height: 420 }}><Story /></div>],
} satisfies Meta<typeof Dialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Confirm: Story = {
  play: async ({ canvas, args }) => {
    const dialogs = canvas.getAllByRole("dialog", { name: 'Delete "Shared hosting"?' });
    await expect(dialogs).toHaveLength(2);
    await expect(dialogs[0]).toHaveAttribute("aria-modal", "true");
    await expect(dialogs[0]).toHaveAccessibleDescription(/removed from Vigie/);
    // Focus starts in the dialog that opened last, and Escape closes it.
    const active = dialogs.find((d) => d.contains(document.activeElement))!;
    await expect(active).toBeDefined();
    await userEvent.keyboard("{Escape}");
    await expect(args.onClose).toHaveBeenCalledOnce();
  },
};

// Tab stays in the dialog: after the last control comes the first.
export const KeepsTheFocus: Story = {
  args: {
    title: "Edit the connection",
    lead: undefined,
    children: <TextField label="Host name" value="ssh.example-host.ch" onChange={() => {}} />,
    actions: (
      <>
        <Button variant="outline">Cancel</Button>
        <Button variant="primary">Save</Button>
      </>
    ),
  },
  play: async ({ canvas }) => {
    const dialog = canvas.getAllByRole("dialog").find((d) => d.contains(document.activeElement))!;
    const field = within(dialog).getByRole("textbox", { name: "Host name" });
    await expect(field).toHaveFocus();
    await userEvent.tab();
    await userEvent.tab();
    await expect(within(dialog).getByRole("button", { name: "Save" })).toHaveFocus();
    await userEvent.tab();
    await expect(field).toHaveFocus();
    await userEvent.tab({ shift: true });
    await expect(within(dialog).getByRole("button", { name: "Save" })).toHaveFocus();
  },
};

// WebKit leaves the focus on the body when a radio or a button is clicked: Escape still closes.
export const EscapeFromTheBody: Story = {
  play: async ({ args }) => {
    (document.activeElement as HTMLElement | null)?.blur();
    await userEvent.keyboard("{Escape}");
    await expect(args.onClose).toHaveBeenCalledOnce();
  },
};
