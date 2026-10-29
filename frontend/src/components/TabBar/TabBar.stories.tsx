import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, userEvent } from "storybook/test";

import { Button } from "../Button/Button";
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

export const WithAction: Story = {
  // Vigie's own button: the browser's default one has no contrast guarantee in the dark theme.
  args: { children: <Button variant="primary">Scan now</Button> },
  play: async ({ canvas }) => {
    // The action sits in the bar, outside the tab list.
    const [action] = canvas.getAllByRole("button", { name: "Scan now" });
    await expect(action!.closest("[role=tablist]")).toBeNull();
  },
};
