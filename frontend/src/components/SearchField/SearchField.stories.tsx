import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { SearchField } from "./SearchField";

const meta = {
  component: SearchField,
  args: { label: "Search", value: "", onChange: fn(), onClose: fn() },
} satisfies Meta<typeof SearchField>;
export default meta;
type Story = StoryObj<typeof meta>;

function Typing({ onClose }: { onClose: () => void }) {
  const [value, setValue] = useState("");
  return <SearchField label="Search" value={value} onChange={setValue} onClose={onClose} />;
}

export const TypeThenEscape: Story = {
  render: (args) => <Typing onClose={args.onClose} />,
  play: async ({ canvas, args }) => {
    const [field] = canvas.getAllByRole("searchbox", { name: "Search" });
    await userEvent.type(field!, "boul");
    await expect(field).toHaveValue("boul");
    // Escape first clears, then closes.
    await userEvent.keyboard("{Escape}");
    await expect(field).toHaveValue("");
    await userEvent.keyboard("{Escape}");
    await expect(args.onClose).toHaveBeenCalled();
  },
};
