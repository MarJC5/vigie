import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { SiteTable } from "./SiteTable";

const meta = {
  component: SiteTable,
  args: {
    label: "Sites of Shared hosting",
    columns: { site: "Site", status: "Status", wordpress: "WordPress", php: "PHP", updates: "Updates" },
    notScanned: "Not scanned yet",
    rows: [
      { id: "a", name: "atelier-exemple.ch", status: "ok", wordpress: "7.1.3", php: "8.4.27", pending: 0 },
      { id: "b", name: "boulangerie-exemple.ch", status: "warn", wordpress: "7.1.2", php: "8.4.27", pending: 4 },
      { id: "c", name: "nouveau.example.test", status: "", wordpress: "", php: "", pending: 0 },
    ],
    onSelect: fn(),
  },
} satisfies Meta<typeof SiteTable>;
export default meta;

export const Sites: StoryObj<typeof meta> = {
  play: async ({ canvas, args }) => {
    const [table] = canvas.getAllByRole("table", { name: "Sites of Shared hosting" });
    await expect(table!.querySelectorAll("tbody tr")).toHaveLength(3);
    await expect(table).toHaveTextContent("Not scanned yet");
    // A site's name opens it.
    const [link] = canvas.getAllByRole("button", { name: "boulangerie-exemple.ch" });
    await userEvent.click(link!);
    await expect(args.onSelect).toHaveBeenCalledWith("b");
  },
};
