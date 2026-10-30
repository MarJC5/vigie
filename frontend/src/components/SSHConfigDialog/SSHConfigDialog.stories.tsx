import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import { sshConfig } from "../../fixtures/data";
import { sampleTree } from "../../fixtures/tree";
import { SSHConfigDialog, type SSHConfigDialogProps } from "./SSHConfigDialog";

function Stateful(args: SSHConfigDialogProps) {
  const [chosen, setChosen] = useState(args.chosen);
  return (
    <SSHConfigDialog
      {...args}
      chosen={chosen}
      onToggle={(alias, on) => {
        const next = new Set(chosen);
        if (on) next.add(alias);
        else next.delete(alias);
        setChosen(next);
        args.onToggle(alias, on);
      }}
    />
  );
}

const meta = {
  component: SSHConfigDialog,
  args: { config: sshConfig, nodes: sampleTree, into: "", onInto: fn(), chosen: new Set<string>(), onToggle: fn(), onImport: fn(), onClose: fn() },
  render: (args) => <Stateful {...args} />,
  decorators: [(Story) => <div style={{ position: "relative", height: 620 }}><Story /></div>],
} satisfies Meta<typeof SSHConfigDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// Nothing checked at first: the file names every server. A host already in Vigie, or one ssh
// cannot read, is shown but not offered.
export const ChooseHosts: Story = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    const add = dialog.getByRole("button", { name: "Add hosts" });
    await expect(add).toBeDisabled();
    await expect(dialog.getAllByRole("checkbox").filter((c) => (c as HTMLInputElement).checked)).toHaveLength(0);
    await expect(dialog.getByRole("checkbox", { name: /shared/ })).toBeDisabled();
    await expect(dialog.getByRole("checkbox", { name: /old-box/ })).toBeDisabled();
    await expect(dialog.getByText("2 entries with patterns (*, ?, !) are skipped: they are not hosts.")).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("checkbox", { name: /atelier/ }));
    await expect(args.onToggle).toHaveBeenCalledWith("atelier", true);
    await userEvent.selectOptions(dialog.getByRole("combobox", { name: "Into" }), "clients");
    await expect(args.onInto).toHaveBeenCalledWith("clients");
    await userEvent.click(dialog.getByRole("button", { name: "Add 1 host" }));
    await expect(args.onImport).toHaveBeenCalled();
  },
};

export const Reading: Story = {
  args: { config: null },
  play: async ({ canvasElement }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=light] [role=dialog]")!);
    await expect(dialog.getByText("Reading ~/.ssh/config…")).toBeInTheDocument();
  },
};

export const NoHost: Story = {
  args: { config: { hosts: [], patterns: 0, problem: null } },
  play: async ({ canvasElement }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getByText("~/.ssh/config names no host.")).toBeInTheDocument();
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  args: { chosen: new Set(["atelier", "vps-perso"]) },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Ajouter 2 hébergements" })).toHaveLength(2);
    await expect(canvas.getAllByText("déjà dans Vigie")).toHaveLength(2);
  },
};
