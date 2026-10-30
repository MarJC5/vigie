import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { SelectField } from "./SelectField";

const options = [
  { value: "", label: "(root)" },
  { value: "clients", label: "Clients" },
  { value: "personal", label: "Personal" },
];

const meta = {
  component: SelectField,
  args: { label: "Folder", value: "clients", options, onChange: fn() },
} satisfies Meta<typeof SelectField>;
export default meta;
type Story = StoryObj<typeof meta>;

function Choosing() {
  const [value, setValue] = useState("");
  return <SelectField label="Folder" value={value} options={options} onChange={setValue} />;
}

export const ChooseAFolder: Story = {
  render: () => <Choosing />,
  play: async ({ canvas }) => {
    const [select] = canvas.getAllByLabelText("Folder");
    await expect(select).toHaveValue("");
    // WebKit draws a native select without the field's fill: the select draws itself.
    await expect(getComputedStyle(select!).appearance).toBe("none");
    await expect(getComputedStyle(select!).backgroundColor).not.toBe("rgba(0, 0, 0, 0)");
    await userEvent.selectOptions(select!, "personal");
    await expect(select).toHaveValue("personal");
  },
};
