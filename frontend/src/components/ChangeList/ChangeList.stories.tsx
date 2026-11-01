import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, within } from "storybook/test";

import { siteChanges } from "../../fixtures/changes";
import { ChangeList } from "./ChangeList";

const meta = {
  component: ChangeList,
  args: { changes: siteChanges, label: "Since the previous scan" },
} satisfies Meta<typeof ChangeList>;
export default meta;
type Story = StoryObj<typeof meta>;

// A line each: what asks for a look with the mark of "needs attention", the rest in gray.
export const Changes: Story = {
  play: async ({ canvasElement }) => {
    const list = within(within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!).getByRole("list", { name: "Since the previous scan" }));
    await expect(list.getAllByRole("listitem")).toHaveLength(8);
    await expect(list.getAllByRole("img", { name: "Worth a look" })).toHaveLength(4);
    await expect(list.getAllByRole("img", { name: "Change" })).toHaveLength(4);
  },
};
