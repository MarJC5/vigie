import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent } from "storybook/test";

import { TabBar, type Tab } from "./TabBar";

const tabs: Tab[] = [
  { id: "overview", label: "Overview" },
  { id: "updates", label: "Updates", count: 3 },
  { id: "security", label: "Security" },
  { id: "notes", label: "Notes" },
  { id: "history", label: "History" },
];

function Controlled() {
  const [value, setValue] = useState("overview");
  return <TabBar label="Site tabs" tabs={tabs} value={value} onChange={setValue} />;
}

const meta = { component: TabBar, args: { label: "Site tabs", tabs, value: "overview", onChange: () => {} } } satisfies Meta<typeof TabBar>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SiteTabs: Story = {
  render: () => <Controlled />,
  play: async ({ canvas }) => {
    const [overview] = canvas.getAllByRole("tab", { name: "Overview" });
    await expect(overview).toHaveAttribute("aria-selected", "true");
    // Arrow keys move between tabs (WAI-ARIA tabs pattern).
    overview!.focus();
    await userEvent.keyboard("{ArrowRight}");
    const [updates] = canvas.getAllByRole("tab", { name: /Updates/ });
    await expect(updates).toHaveFocus();
    await expect(updates).toHaveAttribute("aria-selected", "true");
    await expect(overview).toHaveAttribute("tabindex", "-1");
  },
};
