import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { outdatedSite } from "../../fixtures/scans";
import { failed, preview, running } from "../../fixtures/updates";
import { namesOf, plannedItems, runRows, selectionOf } from "../../lib/updates";
import { RunSteps } from "./RunSteps";

const planned = plannedItems(preview, selectionOf(["plugin:akismet", "plugin:woocommerce", "languages"]));
const names = namesOf(outdatedSite.scan!.updates!);

const meta = {
  component: RunSteps,
  args: { label: "Steps of the update", rows: runRows(running, planned), names, ended: false },
} satisfies Meta<typeof RunSteps>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Running: Story = {
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Steps of the update" });
    const current = list!.querySelector('[aria-current="step"]');
    await expect(current).toHaveTextContent("WooCommerce 9.8.1 → 10.2.0");
    await expect(canvas.getAllByRole("img", { name: "In progress" })).toHaveLength(2);
    await expect(canvas.getAllByRole("img", { name: "Done" })).toHaveLength(10);
    // The translations are still to come.
    await expect(list!.lastElementChild).toHaveTextContent("Translations");
    await expect(list).toHaveTextContent("Database backup");
    await expect(list).toHaveTextContent("6 s");
  },
};

export const Failed: Story = {
  args: { rows: runRows(failed, planned), ended: true },
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Steps of the update" });
    await expect(canvas.getAllByRole("img", { name: "Failed" })).toHaveLength(2);
    await expect(list).toHaveTextContent("wc_get_page_id");
    await expect(list!.lastElementChild).toHaveTextContent("not started");
  },
};
