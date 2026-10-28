import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, userEvent } from "storybook/test";

import { sampleTree } from "../../fixtures/tree";
import { AppShell } from "./AppShell";

// Each story renders once per theme, so the window landmarks repeat by construction;
// the end-to-end test checks that the window has one of each.
const twoThemes = {
  a11y: {
    config: {
      rules: [
        { id: "landmark-unique", enabled: false },
        { id: "landmark-no-duplicate-main", enabled: false },
      ],
    },
  },
};

const meta = {
  component: AppShell,
  args: { nodes: sampleTree },
  parameters: { layout: "fullscreen", ...twoThemes },
  decorators: [
    (Story) => (
      <div style={{ height: 760 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AppShell>;
export default meta;
type Story = StoryObj<typeof meta>;

export const WithSites: Story = {
  play: async ({ canvas }) => {
    const [item] = canvas.getAllByRole("treeitem", {
      name: /reseau-ecoles.ch/,
    });
    await userEvent.click(item!);
    const [heading] = canvas.getAllByRole("heading", {
      level: 1,
      name: "reseau-ecoles.ch",
    });
    await expect(heading).toBeInTheDocument();
    await expect(canvas.getAllByText("Multisite network").length).toBeGreaterThan(0);
  },
};

export const FirstLaunch: Story = {
  args: { nodes: [] },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1, name: "Add your first host" })).toHaveLength(2);
    // Adding a host comes with the next step; the button is shown but not active yet.
    const [add] = canvas.getAllByRole("button", { name: "Add a host" }).filter((b) => b.textContent);
    await expect(add).toBeDisabled();
  },
};
