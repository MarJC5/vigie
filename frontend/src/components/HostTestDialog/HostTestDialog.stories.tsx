import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { HostDetails, TestResult } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { HostTestDialog } from "./HostTestDialog";

const shared: HostDetails = {
  name: "Shared hosting", alias: "", hostname: "ssh.example-host.ch", port: 22, user: "u12ab", keyPath: "", wpCli: "wp", folderId: "clients", maxConcurrent: 0,
};
const passed: TestResult = { phpVersion: "8.4.27", wpCli: "wp", wpCliVersion: "2.12.0", problem: null };
const timedOut: TestResult = {
  phpVersion: "", wpCli: "", wpCliVersion: "",
  problem: { code: "failure.timedOut", detail: "ssh: connect to host ssh.example-host.ch port 22: Operation timed out" },
};

const meta = {
  component: HostTestDialog,
  args: { name: "Shared hosting", details: async () => shared, test: fn(async () => passed), onClose: fn(), onHelp: fn() },
  decorators: [
    (Story) => (
      <div style={{ position: "relative", height: 680 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof HostTestDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// The test runs as the dialog opens: SSH, PHP, WP-CLI.
export const Passed: Story = {
  play: async ({ canvas, args }) => {
    await waitFor(() => expect(args.test).toHaveBeenCalledWith(shared));
    const [checks] = await canvas.findAllByRole("list", { name: "Checks" });
    await waitFor(() => expect(checks).toHaveTextContent("8.4.27"));
    await expect(checks).toHaveTextContent("2.12.0 (wp)");
    // Once per theme as they open, then once more.
    const [again] = canvas.getAllByRole("button", { name: "Test again" });
    const before = (args.test as ReturnType<typeof fn>).mock.calls.length;
    await userEvent.click(again!);
    await waitFor(() => expect(args.test).toHaveBeenCalledTimes(before + 1));
  },
};

// SSH does not answer: the next checks wait, the message says why, the help is one click away.
export const TimedOut: Story = {
  args: { test: fn(async () => timedOut) },
  play: async ({ canvas, args }) => {
    const [alert] = await canvas.findAllByRole("alert");
    await expect(alert).toHaveTextContent("The server did not answer");
    const [help] = canvas.getAllByRole("button", { name: "See the help" });
    await userEvent.click(help!);
    await expect(args.onHelp).toHaveBeenCalled();
  },
};
