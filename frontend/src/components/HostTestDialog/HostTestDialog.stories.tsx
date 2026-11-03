import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { HostDetails, TestResult } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { HostTestDialog } from "./HostTestDialog";

const shared: HostDetails = {
  name: "Shared hosting", alias: "", hostname: "ssh.example-host.ch", port: 22, user: "u12ab", keyPath: "", wpCli: "wp", folderId: "clients", maxConcurrent: 0,
};
const passed: TestResult = { phpVersion: "8.4.27", wpCli: "wp", wpCliVersion: "2.12.0", install: null, problem: null };
const timedOut: TestResult = {
  phpVersion: "", wpCli: "", wpCliVersion: "", install: null,
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

const noWpCli: TestResult = {
  phpVersion: "8.4.27", wpCli: "", wpCliVersion: "", install: { version: "2.12.0", size: 7142777, path: "~/bin/wp" },
  problem: { code: "problem.noWpCli", detail: "wp, wp-cli, ~/bin/wp, php ~/wp-cli.phar" },
};

// No WP-CLI on the account: Vigie offers to install it, then the test passes with ~/bin/wp, which
// the host keeps (ADR-0039).
export const InstallWPCLI: Story = {
  args: {
    test: fn(async () => noWpCli),
    install: fn(async () => ({ command: "~/bin/wp", version: "2.12.0", failedStep: "", problem: null })),
    onInstalled: fn(),
  },
  play: async ({ canvas, args }) => {
    const [offer] = await canvas.findAllByRole("button", { name: "Install WP-CLI…" });
    await userEvent.click(offer!);
    const [confirm] = await canvas.findAllByRole("button", { name: "Install WP-CLI" });
    await userEvent.click(confirm!);
    await waitFor(() => expect(args.onInstalled).toHaveBeenCalledWith("~/bin/wp"));
    await expect(args.install).toHaveBeenCalledWith(shared);
    const [checks] = canvas.getAllByRole("list", { name: "Checks" });
    await expect(checks).toHaveTextContent("2.12.0 (~/bin/wp), installed by Vigie");
    await expect(canvas.getAllByText("WP-CLI is installed")[0]).toBeInTheDocument();
  },
};

// Where no install can be run, nothing is offered: the test says WP-CLI was not found.
export const WithoutTheOffer: Story = {
  args: { test: fn(async () => noWpCli) },
  play: async ({ canvas }) => {
    const [alert] = await canvas.findAllByRole("alert");
    await expect(alert).toHaveTextContent("WP-CLI was not found on this account");
    await expect(canvas.queryByRole("button", { name: "Install WP-CLI…" })).toBeNull();
  },
};
