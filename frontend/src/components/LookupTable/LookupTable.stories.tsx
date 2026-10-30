import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { LookupTable } from "./LookupTable";

const meta = {
  component: LookupTable,
  args: {
    destinations: [
      { id: "wpvulnerability", on: true },
      { id: "wordpress.org", on: true },
      { id: "registry", on: false },
      { id: "site", on: true },
    ],
    onChange: fn(),
  },
} satisfies Meta<typeof LookupTable>;
export default meta;
type Story = StoryObj<typeof meta>;

// Five rows, four switches: IANA follows the domain registries.
export const Services: Story = {
  play: async ({ canvasElement, args }) => {
    const table = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(table.getAllByRole("row")).toHaveLength(6);
    await expect(table.getAllByRole("switch")).toHaveLength(4);
    await userEvent.click(table.getByRole("switch", { name: "wordpress.org" }));
    await expect(args.onChange).toHaveBeenCalledWith("wordpress.org", false);
  },
};

export const ExplainsWhatIsOff: Story = {
  args: { explainOff: true },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Off: domain expiry is not checked.")).toHaveLength(2);
  },
};
