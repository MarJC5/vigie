import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sampleTree } from "../../fixtures/tree";
import { fold } from "../../lib/text";
import { TagManyDialog } from "./TagManyDialog";

const sites = sampleTree.filter((n) => n.kind === "site");

// The dialog over sites that take the tags as the Go side would.
function Tagging({ onTag }: { onTag: (tag: string, add: boolean) => void }) {
  const [nodes, setNodes] = useState<Node[]>(sites);
  return (
    <TagManyDialog
      sites={nodes}
      onTag={async (tag, add) => {
        onTag(tag, add);
        setNodes((ns) =>
          ns.map((n) => {
            const kept = (n.tags ?? []).filter((x) => fold(x) !== fold(tag));
            return { ...n, tags: add ? [...kept, tag] : kept };
          }),
        );
        return null;
      }}
      onClose={() => {}}
    />
  );
}

const meta = {
  component: TagManyDialog,
  args: { sites, onTag: fn(async () => null), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 520 }}><Story /></div>],
} satisfies Meta<typeof TagManyDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// The tags of the three sites, each with how many have it; one added to all, one removed.
export const SeveralSites: Story = {
  render: (args) => <Tagging onTag={args.onTag} />,
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Tags of 3 sites" })[0]!;
    await expect(within(dialog).getAllByRole("listitem").map((i) => i.querySelector("span")?.textContent)).toEqual(["Client Dupont", "Prioritaire"]);
    await expect(within(dialog).getByText("on 2 of the 3 sites")).toBeInTheDocument();
    await userEvent.type(within(dialog).getByRole("textbox", { name: "New tag" }), "WooCommerce{Enter}");
    await expect(args.onTag).toHaveBeenCalledWith("WooCommerce", true);
    await expect(within(dialog).getByText("on the 3 sites")).toBeInTheDocument();
    await expect(within(dialog).getByRole("textbox", { name: "New tag" })).toHaveValue("");
    await userEvent.click(within(dialog).getByRole("button", { name: "Remove Prioritaire from these sites" }));
    await expect(args.onTag).toHaveBeenCalledWith("Prioritaire", false);
    await expect(within(dialog).queryByText("Prioritaire")).not.toBeInTheDocument();
  },
};
