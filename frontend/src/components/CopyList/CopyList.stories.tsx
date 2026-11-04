import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { complete, copiesFolder, database, incomplete, run, runningCopy } from "../../fixtures/copies";
import { CopyList } from "./CopyList";

const meta = {
  component: CopyList,
  args: { copies: [complete, database, incomplete], folder: copiesFolder, onShow: fn(), onResume: fn(), onRemove: fn(), onStop: fn() },
} satisfies Meta<typeof CopyList>;
export default meta;
type Story = StoryObj<typeof meta>;

const dark = (el: HTMLElement) => within(el.querySelector<HTMLElement>("[data-theme=dark]")!);

// Mockup 28e: the copies, complete and incomplete; Show the folder, Resume, the trash alone.
export const History: Story = {
  play: async ({ canvasElement, args }) => {
    const list = dark(canvasElement);
    await expect(list.getByText(copiesFolder)).toBeInTheDocument();
    const rows = list.getAllByRole("listitem");
    await expect(rows).toHaveLength(3);
    await expect(rows[0]).toHaveTextContent(/The database and wp-content8 archives2\.18 GBComplete/);
    await expect(rows[2]).toHaveTextContent(/6 archives of 8.*Incomplete/);
    await userEvent.click(within(rows[0]!).getByRole("button", { name: "Show the folder" }));
    await expect(args.onShow).toHaveBeenCalledWith(complete);
    await userEvent.click(within(rows[2]!).getByRole("button", { name: "Resume" }));
    await expect(args.onResume).toHaveBeenCalledWith(incomplete);
    await userEvent.click(within(rows[1]!).getByRole("button", { name: "Remove the copy" }));
    await expect(args.onRemove).toHaveBeenCalledWith(database);
  },
};

// Mockup 28f: an incomplete copy opened, its missing archives, Resume and the trash in its foot.
export const Incomplete: Story = {
  args: { copies: [complete, incomplete] },
  play: async ({ canvasElement }) => {
    const list = dark(canvasElement);
    await userEvent.click(list.getByRole("button", { name: "6 archives of 8" }));
    await expect(list.getByRole("button", { name: "6 archives of 8" })).toHaveAttribute("aria-expanded", "true");
    await expect(list.getAllByText("Missing")).toHaveLength(2);
    await expect(list.getByText(/Resume fetches the 2 missing archives/)).toBeInTheDocument();
  },
};

// Mockup 28d: a copy running, archive by archive, its bar, Stop; nothing else can start.
export const Running: Story = {
  args: { copies: [runningCopy, database], run },
  play: async ({ canvasElement, args }) => {
    const list = dark(canvasElement);
    await expect(list.getAllByText("Copied")).toHaveLength(5);
    await expect(list.getByRole("progressbar", { name: "Reading · 212 MB of 498 MB" })).toHaveAttribute("aria-valuenow", "212000000");
    await expect(list.getAllByText("Waiting")).toHaveLength(2);
    await userEvent.click(list.getByRole("button", { name: "Stop" }));
    await expect(args.onStop).toHaveBeenCalled();
  },
};

// No copy yet.
export const None: Story = {
  args: { copies: [] },
  play: async ({ canvasElement }) => {
    await expect(dark(canvasElement).getByText("No copy of this site yet.")).toBeInTheDocument();
  },
};
