import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { ExternalLink } from "./ExternalLink";

const meta = {
  component: ExternalLink,
  args: { href: "https://wordpress.org/plugins/akismet/", onOpen: fn(), children: "Akismet" },
} satisfies Meta<typeof ExternalLink>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Accent: Story = {
  play: async ({ canvas, args }) => {
    const [link] = canvas.getAllByRole("link", { name: "Akismet" });
    await userEvent.click(link!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://wordpress.org/plugins/akismet/");
  },
};

// A long name that wraps: the icon keeps its size at the end of the last line.
export const LongNameInAColumn: Story = {
  args: { tone: "text", size: "body", children: "Easy HTTPS (SSL) Redirection" },
  decorators: [
    (Story) => (
      <div style={{ width: 205 }}>
        <Story />
      </div>
    ),
  ],
  play: async ({ canvasElement }) => {
    for (const svg of canvasElement.querySelectorAll("a svg")) {
      const box = svg.getBoundingClientRect();
      await expect([Math.round(box.width), Math.round(box.height)]).toEqual([14, 14]); // the size sm
      // Never alone on its line: on the line of the last word.
      const word = svg.closest("a")!.querySelector("span")!.getBoundingClientRect();
      await expect(Math.abs(box.top + box.height / 2 - (word.top + word.height / 2))).toBeLessThan(word.height / 2);
    }
  },
};
