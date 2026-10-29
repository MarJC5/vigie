import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { Notice } from "./Notice";

const meta = {
  component: Notice,
  args: {
    title: "The server did not answer",
    text: "Some hosts only accept SSH from allowed IP addresses (o2switch, for example): allow yours in the host's control panel.",
    detail: "ssh: connect to host ssh.example-host.ch port 22: Operation timed out",
    action: { label: "Try again", onClick: fn() },
  },
} satisfies Meta<typeof Notice>;
export default meta;

export const Problem: StoryObj<typeof meta> = {
  play: async ({ canvas, args }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("The server did not answer");
    await expect(alert).toHaveTextContent("Operation timed out");
    const [retry] = canvas.getAllByRole("button", { name: "Try again" });
    await userEvent.click(retry!);
    await expect(args.action!.onClick).toHaveBeenCalled();
  },
};
