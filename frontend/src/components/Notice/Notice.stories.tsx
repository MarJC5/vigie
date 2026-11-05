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

export const Warning: StoryObj<typeof meta> = {
  args: {
    title: "WordPress itself is updated",
    text: "The site shows the maintenance page of WordPress for about a minute.",
    detail: undefined,
    action: undefined,
    tone: "warn",
    role: "note",
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryAllByRole("alert")).toHaveLength(0);
    const [note] = canvas.getAllByRole("note");
    await expect(note).toHaveAttribute("data-tone", "warn");
  },
};

// What changed as one types (a host's new address): read politely, without interrupting.
export const Status: StoryObj<typeof meta> = {
  args: {
    tone: "warn",
    role: "status",
    title: "The address changed",
    text: "The key of ssh.new-host.example is checked at the next connection, as for a new host.",
  },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("status")[0]).toHaveTextContent("The address changed");
  },
};

// What to know that is no problem: in gray, its own icon, read in its place.
export const Neutral: StoryObj<typeof meta> = {
  args: {
    tone: "neutral",
    icon: "lock",
    role: "note",
    title: "A sensitive file",
    text: "It names your servers, your SSH users and holds your notes. Keys and passwords are never in it.",
  },
  play: async ({ canvas }) => {
    await expect(canvas.queryAllByRole("alert")).toHaveLength(0);
  },
};
