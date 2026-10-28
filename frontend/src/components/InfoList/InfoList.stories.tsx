import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Button } from "../Button/Button";
import { StatusDot } from "../StatusDot/StatusDot";
import { InfoList, InfoRow } from "./InfoList";

const meta = { component: InfoList } satisfies Meta<typeof InfoList>;
export default meta;
type Story = StoryObj<typeof meta>;

export const SiteOverview: Story = {
  args: {
    children: (
      <>
        <InfoRow label="WordPress">
          7.1.3 <StatusDot status="ok" withLabel />
        </InfoRow>
        <InfoRow label="PHP (web · CLI)">8.4.27 · 8.4.27</InfoRow>
        <InfoRow label="Plugins" action={<Button variant="text">Update</Button>}>
          12 · 2 updates
        </InfoRow>
        <InfoRow label="Path on the server">~/sites/atelier-exemple.ch</InfoRow>
      </>
    ),
  },
  play: async ({ canvas }) => {
    const terms = canvas.getAllByRole("term");
    await expect(terms).toHaveLength(8);
    await expect(terms[0]).toHaveTextContent("WordPress");
    await expect(canvas.getAllByRole("definition")[0]).toHaveTextContent("7.1.3");
  },
};
