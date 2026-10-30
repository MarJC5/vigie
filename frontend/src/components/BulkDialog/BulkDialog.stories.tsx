import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { preview } from "../../fixtures/updates";
import { BulkDialog } from "./BulkDialog";

const previews = [
  preview,
  { ...preview, siteId: "a", name: "atelier-exemple.ch", core: null, plugins: [], themes: [], languages: [] },
  { ...preview, siteId: "e", name: "vieux-site.ch", dumpTool: false },
  { ...preview, siteId: "f", name: "broken.example.test", problem: { code: "problem.cannotLoad", detail: "Error establishing a database connection" } },
];

const meta = {
  component: BulkDialog,
  args: { name: "Shared hosting", previews, onConfirm: fn(), onClose: fn() },
  decorators: [
    (Story) => (
      <div style={{ position: "relative", height: 560 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof BulkDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Sites: Story = {
  play: async ({ canvas, args }) => {
    const [list] = canvas.getAllByRole("list", { name: "Sites" });
    await expect(list).toHaveTextContent("boulangerie-exemple.ch4 items");
    await expect(list).toHaveTextContent("atelier-exemple.chUp to date");
    await expect(list).toHaveTextContent("mariadb-dump or mysqldump");
    await expect(list).toHaveTextContent("WordPress cannot be loaded: Error establishing a database connection");
    await userEvent.click(canvas.getAllByRole("button", { name: "Update 1 site" })[0]!);
    await expect(args.onConfirm).toHaveBeenCalled();
  },
};

export const Loading: Story = {
  args: { previews: null },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("progressbar", { name: "Vigie reads on the server what each site will change." })).toHaveLength(2);
    const [confirm] = canvas.getAllByRole("button", { name: "Update all" });
    await expect(confirm).toBeDisabled();
    await expect(confirm).toHaveAttribute("aria-busy", "true");
  },
};
