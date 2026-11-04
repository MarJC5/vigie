import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import { everything, oneHost } from "../../fixtures/data";
import { sampleTree } from "../../fixtures/tree";
import { ExportDialog, type ExportDialogProps } from "./ExportDialog";

// The choices live in the caller; the story keeps them like the window does.
function Stateful(args: ExportDialogProps) {
  const [nodeId, setNodeId] = useState(args.nodeId);
  const [history, setHistory] = useState(args.history);
  return (
    <ExportDialog
      {...args}
      nodeId={nodeId}
      history={history}
      summary={nodeId === "" ? args.summary : oneHost}
      onNode={(id) => {
        setNodeId(id);
        args.onNode(id);
      }}
      onHistory={(on) => {
        setHistory(on);
        args.onHistory(on);
      }}
    />
  );
}

const meta = {
  component: ExportDialog,
  args: { nodes: sampleTree, nodeId: "", onNode: fn(), history: false, onHistory: fn(), summary: everything, onExport: fn(), onClose: fn() },
  render: (args) => <Stateful {...args} />,
  decorators: [(Story) => <div style={{ position: "relative", height: 620 }}><Story /></div>],
} satisfies Meta<typeof ExportDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// Everything by default, with what it holds; the history says how much it adds; the file is said
// to be sensitive.
export const Everything: Story = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getByText("3 folders, 6 hosts, 11 sites, 4 findings set aside")).toBeInTheDocument();
    await expect(dialog.getByText(/about 3\.2 MB more\.$/)).toBeInTheDocument();
    await expect(dialog.getByText("A sensitive file")).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("checkbox", { name: /Include the history/ }));
    await expect(args.onHistory).toHaveBeenCalledWith(true);
    await userEvent.click(dialog.getByRole("button", { name: "Export" }));
    await expect(args.onExport).toHaveBeenCalled();
  },
};

export const AFolderOrAHost: Story = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=light] [role=dialog]")!);
    await userEvent.click(dialog.getByRole("radio", { name: /A folder or a host/ }));
    await expect(args.onNode).toHaveBeenCalledWith("clients");
    await userEvent.selectOptions(dialog.getByRole("combobox", { name: "A folder or a host" }), "shared");
    await expect(args.onNode).toHaveBeenLastCalledWith("shared");
    await expect(dialog.getByText("1 host, 3 sites")).toBeInTheDocument();
  },
};

export const Failed: Story = {
  args: { problem: { code: "problem.error", detail: "open /Volumes/Backup/vigie.json: permission denied" } },
  play: async ({ canvasElement }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getByRole("alert")).toHaveTextContent("Not exported");
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("3 dossiers, 6 hébergements, 11 sites, 4 constats mis de côté")).toHaveLength(2);
    await expect(canvas.getAllByText(/environ 3,2\sMo de plus\.$/)).toHaveLength(2);
  },
};
