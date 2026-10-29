import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { type InstallItem, InstallList } from "./InstallList";

const items: InstallItem[] = [
  { path: "~/sites/plain", name: "plain.example.test", state: "New", detail: "https://plain.example.test", checked: true, selectable: true },
  { path: "~/sites/network", name: "network.example.test", state: "New multisite network", detail: "4 subsites", checked: true, selectable: true },
  { path: "~/sites/cabinet", name: "cabinet-exemple.ch", state: "Already in Vigie", detail: "", checked: false, selectable: false },
  { path: "~/sites/broken", name: "broken", state: "Cannot be loaded", detail: "Error establishing a database connection.", checked: false, selectable: false, problem: true },
];

const columns = { add: "Add", name: "Name", path: "Path on the server", state: "State" };

const meta = {
  component: InstallList,
  args: { label: "Installs found", columns, addLabel: "Add", items, onToggle: fn(), onRename: fn() },
} satisfies Meta<typeof InstallList>;
export default meta;
type Story = StoryObj<typeof meta>;

function Editable() {
  const [list, setList] = useState(items);
  return (
    <InstallList
      label="Installs found"
      columns={columns}
      addLabel="Add"
      items={list}
      onToggle={(path) => setList((l) => l.map((i) => (i.path === path ? { ...i, checked: !i.checked } : i)))}
      onRename={(path, name) => setList((l) => l.map((i) => (i.path === path ? { ...i, name } : i)))}
    />
  );
}

export const Found: Story = {
  render: () => <Editable />,
  play: async ({ canvas }) => {
    const [plain] = canvas.getAllByRole("checkbox", { name: "Add plain.example.test" });
    await expect(plain).toBeChecked();
    await userEvent.click(plain!);
    await expect(plain).not.toBeChecked();
    // Only new installs can be added; the others are shown, not offered.
    await expect(canvas.queryAllByRole("checkbox", { name: "Add cabinet-exemple.ch" })).toHaveLength(0);
    const [name] = canvas.getAllByRole("textbox", { name: "Name ~/sites/network" });
    await userEvent.clear(name!);
    await userEvent.type(name!, "Écoles");
    await expect(name).toHaveValue("Écoles");
    // A problem is marked by a shape, never by color alone.
    const [broken] = canvas.getAllByText("Cannot be loaded");
    await expect(broken!.closest("td")!.querySelector("[data-shape='triangle']")).not.toBeNull();
  },
};
