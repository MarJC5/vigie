import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Column, Columns } from "./Columns";

const meta = {
  component: Columns,
  args: {
    count: 6,
    children: (
      <>
        <Column span={3}>
          <p>Name</p>
        </Column>
        <Column span={3}>
          <p>Folder</p>
        </Column>
        <Column span={4}>
          <p>Host name</p>
        </Column>
        <Column span={2}>
          <p>Port</p>
        </Column>
      </>
    ),
  },
} satisfies Meta<typeof Columns>;
export default meta;

export const FormRows: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    const [name] = canvas.getAllByText("Name");
    const [folder] = canvas.getAllByText("Folder");
    // Two halves side by side.
    await expect(name!.getBoundingClientRect().top).toBe(folder!.getBoundingClientRect().top);
    await expect(folder!.getBoundingClientRect().left).toBeGreaterThan(name!.getBoundingClientRect().left);
  },
};
