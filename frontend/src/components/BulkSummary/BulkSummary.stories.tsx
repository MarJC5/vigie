import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { bulk } from "../../fixtures/updates";
import { BulkSummary } from "./BulkSummary";

const meta = {
  component: BulkSummary,
  args: { results: bulk, onOpen: fn() },
} satisfies Meta<typeof BulkSummary>;
export default meta;
type Story = StoryObj<typeof meta>;

export const StoppedAtASite: Story = {
  play: async ({ canvas, args }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("The update stopped at boulangerie-exemple.ch");
    await userEvent.click(canvas.getAllByRole("button", { name: "Open boulangerie-exemple.ch" })[0]!);
    await expect(args.onOpen).toHaveBeenCalledWith("outdated");
    const [counts] = canvas.getAllByRole("list", { name: "Summary" });
    await expect(counts).toHaveTextContent("Updated1Failed1Not started2Refused1");
    const [table] = canvas.getAllByRole("table", { name: "Result by site" });
    await expect(table!.querySelectorAll("tbody tr")).toHaveLength(5);
    await expect(table).toHaveTextContent("mariadb-dump or mysqldump");
  },
};

export const AllUpdated: Story = {
  args: { results: bulk.slice(0, 1) },
  play: async ({ canvas }) => {
    await expect(canvas.queryAllByRole("alert")).toHaveLength(0);
  },
};
