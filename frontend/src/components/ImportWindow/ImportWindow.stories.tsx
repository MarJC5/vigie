import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import { csvPreview, vigiePreview } from "../../fixtures/data";
import { sampleTree } from "../../fixtures/tree";
import { ImportWindow, type ImportWindowProps } from "./ImportWindow";

function Stateful(args: ImportWindowProps) {
  const [keep, setKeep] = useState(args.keep);
  return (
    <ImportWindow
      {...args}
      keep={keep}
      onKeep={(id, on) => {
        const next = new Set(keep);
        if (on) next.add(id);
        else next.delete(id);
        setKeep(next);
        args.onKeep(id, on);
      }}
    />
  );
}

// Each story renders once per theme, so the main landmark repeats by construction; the end-to-end
// test checks that the window has one.
const meta = {
  component: ImportWindow,
  parameters: {
    a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }, { id: "landmark-no-duplicate-main", enabled: false }] } },
  },
  args: { preview: vigiePreview, nodes: sampleTree, onInto: fn(), keep: new Set<string>(), onKeep: fn(), onConfirm: fn(), onClose: fn() },
  render: (args) => <Stateful {...args} />,
  decorators: [(Story) => <div style={{ height: 800 }}><Story /></div>],
} satisfies Meta<typeof ImportWindow>;
export default meta;
type Story = StoryObj<typeof meta>;

// What is added, what is already there; a key is kept only once checked, one Vigie cannot take
// says why.
export const AVigieFile: Story = {
  play: async ({ canvasElement, args }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(screen.getByRole("heading", { name: "Import vigie-2026-10-30.json" })).toBeInTheDocument();
    await expect(screen.getByText(/^Exported on 30\.10\.2026 at .* by Vigie 0\.6\.0, with history\./)).toBeInTheDocument();
    await expect(screen.getByRole("heading", { name: "Added 5" })).toBeInTheDocument();
    await expect(screen.getByRole("heading", { name: "Already in Vigie, left as it is 2" })).toBeInTheDocument();
    await expect(screen.getByText("Not kept: Vigie trusts another key for this server.")).toBeInTheDocument();
    const key = screen.getByRole("checkbox", { name: /Clients\/Valais\/Atelier · ssh\.example-host\.ch/ });
    await expect(key).not.toBeChecked();
    await userEvent.click(key);
    await expect(args.onKeep).toHaveBeenCalledWith("h-atelier", true);
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Into" }), "personal");
    await expect(args.onInto).toHaveBeenCalledWith("personal");
    await userEvent.click(screen.getByRole("button", { name: "Add 5 items" }));
    await expect(args.onConfirm).toHaveBeenCalled();
  },
};

export const ACSVList: Story = {
  args: { preview: csvPreview },
  play: async ({ canvasElement }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(screen.getByText(/^14 lines read, separated by semicolons\./)).toBeInTheDocument();
    await expect(screen.getByText("Line 6: no path")).toBeInTheDocument();
    await expect(screen.queryByRole("heading", { name: "Server keys" })).not.toBeInTheDocument();
  },
};

export const NothingNew: Story = {
  args: { preview: { ...vigiePreview, items: vigiePreview.items!.filter((i) => i.status === "present"), keys: [], added: 0 } },
  play: async ({ canvasElement }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(screen.getByText("Nothing to add: everything is already in Vigie.")).toBeInTheDocument();
    await expect(screen.getByRole("button", { name: "Add 0 items" })).toBeDisabled();
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Ajouter 5 éléments" })).toHaveLength(2);
    await expect(canvas.getAllByText(/^site · ~\/www\/atelier · avec 12 relevés, 3 mises à jour et retours en arrière$/)).toHaveLength(2);
  },
};
