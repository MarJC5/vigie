import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { SiteTable } from "./SiteTable";

const meta = {
  component: SiteTable,
  args: {
    label: "Sites of Shared hosting",
    columns: { site: "Site", status: "Status", worst: "Worst finding", updates: "Updates" },
    notScanned: "Not scanned yet",
    rows: [
      { id: "a", name: "atelier-exemple.ch", status: "ok", worst: "", pending: 0 },
      { id: "b", name: "boulangerie-exemple.ch", status: "danger", worst: "WooCommerce 9.8.1: SQL injection without authentication", pending: 4 },
      { id: "v", name: "vieux-site.ch", status: "partial", worst: "", pending: 0 },
      { id: "c", name: "nouveau.example.test", status: "", worst: "", pending: 0 },
    ],
    onSelect: fn(),
  },
} satisfies Meta<typeof SiteTable>;
export default meta;

export const Sites: StoryObj<typeof meta> = {
  play: async ({ canvas, args }) => {
    const [table] = canvas.getAllByRole("table", { name: "Sites of Shared hosting" });
    await expect(table!.querySelectorAll("tbody tr")).toHaveLength(4);
    await expect(table).toHaveTextContent("WooCommerce 9.8.1: SQL injection without authentication");
    await expect(table).toHaveTextContent("Not fully checked");
    await expect(table).toHaveTextContent("Not scanned yet");
    // A site's name opens it.
    const [link] = canvas.getAllByRole("button", { name: "boulangerie-exemple.ch" });
    await userEvent.click(link!);
    await expect(args.onSelect).toHaveBeenCalledWith("b");
  },
};
