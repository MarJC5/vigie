import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { TestResult, WPCLIInstallProgress, WPCLIInstallResult } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { WpCliInstallDialog, WpCliNotice } from "./WpCliInstall";

const offer = { version: "2.12.0", size: 7142777, path: "~/bin/wp" };
const noWpCli: TestResult = {
  phpVersion: "8.4.27", wpCli: "", wpCliVersion: "", install: offer,
  problem: { code: "problem.noWpCli", detail: "wp, wp-cli, ~/bin/wp, php ~/wp-cli.phar" },
};

const meta = {
  component: WpCliInstallDialog,
  args: {
    offer,
    name: "Shared hosting",
    account: "u12ab@ssh.example-host.ch",
    install: fn(async (): Promise<WPCLIInstallResult> => ({ command: "~/bin/wp", version: "2.12.0", failedStep: "", problem: null })),
    onInstalled: fn(),
    onClose: fn(),
  },
  decorators: [
    (Story) => (
      <div style={{ position: "relative", height: 640 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof WpCliInstallDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// What is written, where it comes from, how it is checked, how to remove it.
export const Confirm: Story = {
  play: async ({ canvas, args }) => {
    const [dialog] = canvas.getAllByRole("dialog", { name: "Install WP-CLI on Shared hosting?" });
    await expect(dialog).toHaveTextContent("u12ab@ssh.example-host.ch");
    await expect(dialog).toHaveTextContent("SHA-512 digest built into Vigie, before anything is sent");
    await expect(dialog).toHaveTextContent("rm ~/bin/wp");
    await expect(args.install).not.toHaveBeenCalled();
  },
};

// Confirmed, it installs, then tells the command that answered.
export const Installs: Story = {
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("button", { name: "Install WP-CLI" })[0]!);
    await waitFor(() => expect(args.onInstalled).toHaveBeenCalledWith(expect.objectContaining({ command: "~/bin/wp" })));
  },
};

// The install under way: each step, the bytes sent; it cannot be closed meanwhile.
let tell: (p: WPCLIInstallProgress) => void = () => {};
export const Installing: Story = {
  args: {
    install: fn((): Promise<WPCLIInstallResult> => new Promise(() => {})),
    onProgress: (listener) => {
      tell = listener;
      return () => {};
    },
  },
  play: async ({ canvas }) => {
    await userEvent.click(canvas.getAllByRole("button", { name: "Install WP-CLI" })[0]!);
    await waitFor(() => expect(canvas.getAllByRole("dialog", { name: "Installing WP-CLI" }).length).toBeGreaterThan(0));
    tell({ step: "send", done: 4_200_000, total: 7_142_777 });
    await waitFor(() => expect(canvas.getAllByRole("list", { name: "Steps of the install" })[0]).toHaveTextContent(/4(\.|,)2 MB of 7(\.|,)1 MB/));
    await expect(canvas.getAllByRole("button", { name: "Installing…" })[0]).toBeDisabled();
  },
};

// Stopped at the send: ~/bin/wp appeared meanwhile, and is left alone.
export const Failed: Story = {
  args: {
    install: fn(async (): Promise<WPCLIInstallResult> => ({ command: "", version: "", failedStep: "send", problem: { code: "problem.wpCliExists", detail: "~/bin/wp" } })),
  },
  play: async ({ canvas, args }) => {
    await userEvent.click(canvas.getAllByRole("button", { name: "Install WP-CLI" })[0]!);
    const [alert] = await canvas.findAllByRole("alert");
    await expect(alert).toHaveTextContent("The send failed");
    await expect(alert).toHaveTextContent("Vigie leaves it alone");
    await expect(canvas.getAllByRole("button", { name: "Close" })[0]).toBeEnabled();
    await expect(args.onClose).not.toHaveBeenCalled();
  },
};

// The notices of the test: the offer, PHP too old, ~/bin/wp there, installed.
export const Notices: Story = {
  render: () => (
    <div style={{ display: "flex", flexDirection: "column", gap: 16, maxWidth: 760 }}>
      <WpCliNotice test={noWpCli} installed={false} onInstall={fn()} onRetry={fn()} />
      <WpCliNotice
        test={{ ...noWpCli, phpVersion: "7.1.33", install: null, problem: { code: "problem.wpCliPhpTooOld", detail: "7.1.33" } }}
        installed={false}
        onInstall={fn()}
        onRetry={fn()}
      />
      <WpCliNotice test={{ ...noWpCli, install: null, problem: { code: "problem.wpCliExists", detail: "~/bin/wp" } }} installed={false} onInstall={fn()} onRetry={fn()} />
      <WpCliNotice test={{ ...noWpCli, wpCli: "~/bin/wp", wpCliVersion: "2.12.0", install: null, problem: null }} installed onInstall={fn()} onRetry={fn()} />
    </div>
  ),
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Install WP-CLI…" })).toHaveLength(2);
    await expect(canvas.getAllByText("PHP is too old for WP-CLI")).toHaveLength(2);
    await expect(canvas.getAllByText("~/bin/wp already exists")).toHaveLength(2);
    await expect(canvas.getAllByText("WP-CLI is installed")).toHaveLength(2);
  },
};
