import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { everything, oneHost, sshConfig, vigiePreview } from "../../fixtures/data";
import { sampleTree } from "../../fixtures/tree";
import { type DataApi, DataFlow } from "./DataFlow";

/** A fake Data service: the file dialog answers with the preview, an export with a file. */
const api = (calls: (name: string, ...args: unknown[]) => void): DataApi => ({
  ExportSummary: async (nodeId) => (calls("ExportSummary", nodeId), nodeId ? oneHost : everything),
  Export: async (nodeId, history) => (calls("Export", nodeId, history), { file: "/Users/someone/Documents/vigie-2026-10-30.json", problem: null }),
  ChooseImport: async (kind) => (calls("ChooseImport", kind), vigiePreview),
  PreviewImport: async (into) => (calls("PreviewImport", into), { ...vigiePreview, into }),
  ApplyImport: async (ids) => (calls("ApplyImport", ids), { added: 5, problem: null }),
  CancelImport: async () => void calls("CancelImport"),
  SSHConfig: async () => (calls("SSHConfig"), sshConfig),
  ImportSSHConfig: async (aliases, into) => (calls("ImportSSHConfig", aliases, into), { added: aliases.length, problem: null }),
});

// The preview of an import takes the window: fixed layers, one story renders one theme.
const meta = {
  component: DataFlow,
  parameters: {
    a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }, { id: "landmark-no-duplicate-main", enabled: false }] } },
  },
  args: { api: api(fn()), task: { kind: "export", nodeId: "" }, nodes: sampleTree, onClose: fn() },
  decorators: [(Story) => <div style={{ height: 720, transform: "translateZ(0)" }}><Story /></div>],
} satisfies Meta<typeof DataFlow>;
export default meta;
type Story = StoryObj<typeof meta>;

const calls = fn();

export const ExportEverything: Story = {
  args: { api: api(calls) },
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await waitFor(() => expect(dialog.getByText("3 folders, 6 hosts, 11 sites, 4 findings set aside")).toBeInTheDocument());
    await userEvent.click(dialog.getByRole("button", { name: "Export" }));
    await waitFor(() =>
      expect(args.onClose).toHaveBeenCalledWith({ row: "export", text: "Saved to /Users/someone/Documents/vigie-2026-10-30.json." }),
    );
    await expect(calls).toHaveBeenCalledWith("Export", "", false);
  },
};

export const ImportFromTheSSHConfig: Story = {
  args: { api: api(calls), task: { kind: "ssh" } },
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=light] [role=dialog]")!);
    await userEvent.click(await dialog.findByRole("checkbox", { name: /vps-perso/ }));
    await userEvent.click(dialog.getByRole("button", { name: "Add 1 host" }));
    await waitFor(() => expect(args.onClose).toHaveBeenCalledWith({ row: "import", text: "1 folder, host or site added." }, true));
    await expect(calls).toHaveBeenCalledWith("ImportSSHConfig", ["vps-perso"], "");
  },
};

// The key is kept only once checked; another folder previews again and asks for the keys again.
export const ImportAVigieFile: Story = {
  args: { api: api(calls), task: { kind: "import", source: "vigie" } },
  play: async ({ canvasElement, args }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    const key = await screen.findByRole("checkbox", { name: /ssh\.example-host\.ch/ });
    await userEvent.click(key);
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Into" }), "clients");
    await waitFor(() => expect(calls).toHaveBeenCalledWith("PreviewImport", "clients"));
    await waitFor(() => expect(screen.getByRole("checkbox", { name: /ssh\.example-host\.ch/ })).not.toBeChecked());
    await userEvent.click(screen.getByRole("checkbox", { name: /ssh\.example-host\.ch/ }));
    await userEvent.click(screen.getByRole("button", { name: "Add 5 items" }));
    await waitFor(() => expect(args.onClose).toHaveBeenCalledWith({ row: "import", text: "5 folders, hosts and sites added." }, true));
    await expect(calls).toHaveBeenCalledWith("ApplyImport", ["h-atelier"]);
  },
};
