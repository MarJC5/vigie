import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import type {
  AddResult,
  Choice,
  DiscoverResult,
  HostDetails,
  KeysResult,
  Problem,
  TestResult,
  WPCLIInstallResult,
} from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { AddHostWizard, type WizardApi } from "./AddHostWizard";

const keys: KeysResult = {
  name: "ssh.example-host.ch",
  keys: [
    { type: "ssh-rsa", fingerprint: "SHA256:C7A6jy0J8YO6e3wDBgqIOPDXyuyIe3qo7twDzAIDY2c" },
    { type: "ecdsa-sha2-nistp256", fingerprint: "SHA256:+91qk9uB/U6iayJDcd19LyfUoBfNK///ByvoJTlXrbQ" },
    { type: "ssh-ed25519", fingerprint: "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg" },
  ],
  problem: null,
};

const found: DiscoverResult = {
  installs: [
    { path: "~/sites/plain", state: "new", name: "plain.example.test", url: "https://plain.example.test", multisite: false, subsites: 0, message: "" },
    { path: "~/sites/network", state: "new", name: "network.example.test", url: "https://network.example.test", multisite: true, subsites: 4, message: "" },
    { path: "~/sites/broken", state: "cannot-load", name: "broken", url: "", multisite: false, subsites: 0, message: "Error establishing a database connection." },
  ],
  truncated: false,
  problem: null,
};

const passed: TestResult = { phpVersion: "8.4.27", wpCli: "wp-cli", wpCliVersion: "2.12.0", install: null, problem: null };
const timedOut: TestResult = {
  phpVersion: "",
  wpCli: "",
  wpCliVersion: "", install: null,
  problem: { code: "failure.timedOut", detail: "ssh: connect to host ssh.example-host.ch port 22: Operation timed out" },
};

const offer = { version: "2.12.0", size: 7142777, path: "~/bin/wp" };
const noWpCli: TestResult = {
  phpVersion: "8.4.27", wpCli: "", wpCliVersion: "", install: offer,
  problem: { code: "problem.noWpCli", detail: "wp, wp-cli, ~/bin/wp, php ~/wp-cli.phar" },
};
const installedWpCli: WPCLIInstallResult = { command: "~/bin/wp", version: "2.12.0", failedStep: "", problem: null };

// A fake of the Go service: records the calls, answers as a server would.
function fake(tests: TestResult[] = [passed], install: WPCLIInstallResult = installedWpCli) {
  const calls = {
    trust: [] as Array<[string, string]>,
    added: [] as Array<{ details: HostDetails; fingerprint: string; choices: Choice[] }>,
    installed: [] as Array<{ details: HostDetails; hostId: string }>,
  };
  let test = 0;
  const api: WizardApi = {
    Check: async (): Promise<Problem | null> => null,
    HostKeys: async () => keys,
    Trust: async (name, fingerprint) => {
      calls.trust.push([name, fingerprint]);
      return null;
    },
    TestConnection: async () => tests[Math.min(test++, tests.length - 1)]!,
    Discover: async () => found,
    Add: async (details, fingerprint, choices): Promise<AddResult> => {
      calls.added.push({ details, fingerprint, choices: choices ?? [] });
      return { hostId: "host-1", problem: null };
    },
    InstallWPCLI: async (details, hostId) => {
      calls.installed.push({ details, hostId });
      return install;
    },
  };
  return { api, calls };
}

const folders = [{ id: "clients", path: "Clients" }];

const meta = {
  component: AddHostWizard,
  args: { api: fake().api, folders, onClose: fn(), onAdded: fn() },
  parameters: {
    layout: "fullscreen",
    // Rendered once per theme: the main landmark repeats by construction.
    a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }, { id: "landmark-no-duplicate-main", enabled: false }] } },
  },
  decorators: [
    (Story) => (
      <div style={{ height: 760 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AddHostWizard>;
export default meta;
type Story = StoryObj<typeof meta>;

// The first of the two renders (one per theme).
const first = (canvasElement: HTMLElement) => within(canvasElement.querySelector<HTMLElement>("[data-theme]")!);

const success = fake();

export const AddAHost: Story = {
  args: { api: success.api },
  play: async ({ canvasElement, args }) => {
    const ui = first(canvasElement);
    const { calls } = success;

    await userEvent.type(ui.getByLabelText("Name in Vigie"), "Shared hosting");
    await userEvent.type(ui.getByLabelText("Host name"), "ssh.example-host.ch");
    await userEvent.type(ui.getByLabelText("User"), "u12ab");
    await userEvent.clear(ui.getByLabelText("WP-CLI command"));
    await userEvent.type(ui.getByLabelText("WP-CLI command"), "wp");
    await userEvent.click(ui.getByRole("button", { name: "Continue" }));

    // The ed25519 key is the one shown, and it must be compared before it is trusted.
    await expect(await ui.findByRole("heading", { level: 1, name: "Check the server's key" })).toBeInTheDocument();
    const key = ui.getByRole("group", { name: "Key of ssh.example-host.ch" });
    await expect(key).toHaveTextContent("SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg");
    // Key types by their usual names.
    await expect(key).toHaveTextContent("ED25519");
    await expect(key).toHaveTextContent("ECDSA · SHA256:+91qk9uB");
    await expect(key).toHaveTextContent("RSA · SHA256:C7A6");
    const trust = ui.getByRole("button", { name: "Trust and continue" });
    await expect(trust).toBeDisabled();
    await userEvent.click(ui.getByLabelText("I compared this fingerprint with the one of my host"));
    await userEvent.click(trust);
    await expect(calls.trust).toEqual([["ssh.example-host.ch", "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg"]]);

    // The test runs at once, then the search, with no click; WP-CLI was found under another
    // command, which the host keeps.
    await expect(await ui.findByRole("heading", { level: 1, name: "Choose the sites to add" })).toBeInTheDocument();
    await expect(ui.queryByRole("checkbox", { name: "Add broken" })).toBeNull();
    await userEvent.click(ui.getByRole("checkbox", { name: "Add plain.example.test" }));
    await userEvent.click(ui.getByRole("button", { name: "Add the host and 1 site" }));

    await waitFor(() => expect(calls.added).toHaveLength(1));
    const added = calls.added[0]!;
    await expect(added.details.wpCli).toBe("wp-cli");
    await expect(added.details.folderId).toBe("");
    await expect(added.choices).toEqual([{ path: "~/sites/network", name: "network.example.test", url: "https://network.example.test", multisite: true }]);
    await waitFor(() => expect(args.onAdded).toHaveBeenCalledWith("host-1"));
  },
};

export const ServerDoesNotAnswer: Story = {
  args: { api: fake([timedOut, passed]).api },
  play: async ({ canvasElement }) => {
    const ui = first(canvasElement);
    await userEvent.type(ui.getByLabelText("Name in Vigie"), "o2switch");
    await userEvent.type(ui.getByLabelText("Host name"), "ssh.example-host.ch");
    await userEvent.type(ui.getByLabelText("User"), "u12ab");
    await userEvent.click(ui.getByRole("button", { name: "Continue" }));
    await userEvent.click(await ui.findByLabelText("I compared this fingerprint with the one of my host"));
    await userEvent.click(ui.getByRole("button", { name: "Trust and continue" }));

    const alert = await ui.findByRole("alert");
    await expect(alert).toHaveTextContent("allow yours in the host's control panel");
    await expect(alert).toHaveTextContent("Operation timed out");
    await expect(ui.getByRole("button", { name: "Search for sites" })).toBeDisabled();
    // Once the test passes, the search follows by itself.
    await userEvent.click(ui.getByRole("button", { name: "Try again" }));
    await expect(await ui.findByRole("heading", { level: 1, name: "Choose the sites to add" })).toBeInTheDocument();
  },
};

export const MissingFields: Story = {
  play: async ({ canvasElement }) => {
    const ui = first(canvasElement);
    // Nothing typed yet: Continue waits for a name and a way to connect.
    await expect(ui.getByRole("button", { name: "Continue" })).toBeDisabled();
    await userEvent.click(ui.getByRole("button", { name: /Alias from/ }));
    await expect(ui.getByLabelText("SSH alias")).toBeInTheDocument();
    await expect(ui.queryByLabelText("Host name")).toBeNull();
  },
};

// The search waits for the account: what happens is shown, then the count of installs checked.
const slow = (() => {
  const base = fake();
  let release: () => void = () => {};
  let listener: ((p: { checked: number; total: number }) => void) | undefined;
  const api: WizardApi = {
    ...base.api,
    onDiscovery: (l) => {
      listener = l;
      return () => {
        listener = undefined;
      };
    },
    Discover: () =>
      new Promise((resolve) => {
        release = () => resolve(found);
      }),
  };
  return { api, progress: (checked: number, total: number) => listener?.({ checked, total }), release: () => release() };
})();

export const SearchShowsProgress: Story = {
  args: { api: slow.api },
  play: async ({ canvasElement }) => {
    const ui = first(canvasElement);
    await userEvent.type(ui.getByLabelText("Name in Vigie"), "Bench");
    await userEvent.type(ui.getByLabelText("Host name"), "ssh.example-host.ch");
    await userEvent.type(ui.getByLabelText("User"), "u12ab");
    await userEvent.click(ui.getByRole("button", { name: "Continue" }));
    await userEvent.click(await ui.findByLabelText("I compared this fingerprint with the one of my host"));
    await userEvent.click(ui.getByRole("button", { name: "Trust and continue" }));

    // The search follows the test by itself; its button turns meanwhile.
    await expect(await ui.findByRole("button", { name: "Search for sites" })).toHaveAttribute("aria-busy", "true");
    await expect(await ui.findByRole("progressbar", { name: "Searching the account for WordPress installs…" })).toBeInTheDocument();
    slow.progress(1, 3);
    const bar = await ui.findByRole("progressbar", { name: "Checking the installs found: 1 of 3" });
    await expect(bar).toHaveAttribute("aria-valuenow", "1");
    slow.release();
    await expect(await ui.findByRole("heading", { level: 1, name: "Choose the sites to add" })).toBeInTheDocument();
    await expect(ui.queryByRole("progressbar")).toBeNull();
  },
};

// A search that failed stays on the test, which passed: the button searches again.
const searchFails = (() => {
  const base = fake();
  let searches = 0;
  const api: WizardApi = {
    ...base.api,
    Discover: async () =>
      searches++ === 0 ? { installs: [], truncated: false, problem: { code: "problem.findFailed", detail: "find: Permission denied" } } : found,
  };
  return { api };
})();

export const SearchFailsThenRetried: Story = {
  args: { api: searchFails.api },
  play: async ({ canvasElement }) => {
    const ui = first(canvasElement);
    await userEvent.type(ui.getByLabelText("Name in Vigie"), "Bench");
    await userEvent.type(ui.getByLabelText("Host name"), "ssh.example-host.ch");
    await userEvent.type(ui.getByLabelText("User"), "u12ab");
    await userEvent.click(ui.getByRole("button", { name: "Continue" }));
    await userEvent.click(await ui.findByLabelText("I compared this fingerprint with the one of my host"));
    await userEvent.click(ui.getByRole("button", { name: "Trust and continue" }));

    const alert = await ui.findByRole("alert");
    await expect(alert).toHaveTextContent("The search for sites failed");
    await expect(alert).toHaveTextContent("Permission denied");
    await expect(ui.getByRole("heading", { level: 1 })).toHaveTextContent("Test the connection");
    // Try again searches again, without testing again.
    await userEvent.click(ui.getByRole("button", { name: "Try again" }));
    await expect(await ui.findByRole("heading", { level: 1, name: "Choose the sites to add" })).toBeInTheDocument();
  },
};

// Up to the test step of a host on the account of u12ab.
async function toTheTest(canvasElement: HTMLElement) {
  const ui = first(canvasElement);
  await userEvent.type(ui.getByLabelText("Name in Vigie"), "Shared hosting");
  await userEvent.type(ui.getByLabelText("Host name"), "ssh.example-host.ch");
  await userEvent.type(ui.getByLabelText("User"), "u12ab");
  await userEvent.click(ui.getByRole("button", { name: "Continue" }));
  await userEvent.click(await ui.findByLabelText("I compared this fingerprint with the one of my host"));
  await userEvent.click(ui.getByRole("button", { name: "Trust and continue" }));
  return ui;
}

const installing = fake([noWpCli]);

// No WP-CLI on the account: Vigie offers to install it, says what it writes, then the test passes
// with ~/bin/wp, which the host keeps (ADR-0039).
export const InstallWPCLI: Story = {
  args: { api: installing.api },
  play: async ({ canvasElement, args }) => {
    const ui = await toTheTest(canvasElement);
    await expect(await ui.findByText("WP-CLI is missing on this account")).toBeInTheDocument();
    await expect(ui.getByRole("list", { name: "Checks" })).toHaveTextContent("Not found (tried: wp, wp-cli, ~/bin/wp, php ~/wp-cli.phar)");
    await expect(ui.getByRole("button", { name: "Search for sites" })).toBeDisabled();
    await userEvent.click(ui.getByRole("button", { name: "Install WP-CLI…" }));

    const dialog = within(await within(canvasElement).findByRole("dialog", { name: "Install WP-CLI on Shared hosting?" }));
    await expect(dialog.getByText(/account u12ab@ssh.example-host.ch/)).toBeInTheDocument();
    await expect(dialog.getByText("rm ~/bin/wp")).toBeInTheDocument();
    await expect(dialog.getByText(/^2\.12\.0 · 7(\.|,)1 MB$/)).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Install WP-CLI" }));

    await expect(await ui.findByText("WP-CLI is installed")).toBeInTheDocument();
    await expect(ui.getByRole("list", { name: "Checks" })).toHaveTextContent("2.12.0 (~/bin/wp), installed by Vigie");
    await expect(installing.calls.installed).toHaveLength(1);
    await expect(installing.calls.installed[0]!.hostId).toBe("");
    await userEvent.click(ui.getByRole("button", { name: "Search for sites" }));
    await userEvent.click(await ui.findByRole("button", { name: /Add the host/ }));
    await waitFor(() => expect(args.onAdded).toHaveBeenCalled());
    await expect(installing.calls.added.at(-1)!.details.wpCli).toBe("~/bin/wp");
  },
};

// The file downloaded is not the WP-CLI pinned: the install stops at the check, nothing sent.
export const InstallRefused: Story = {
  args: {
    api: fake([noWpCli], {
      command: "", version: "", failedStep: "check",
      problem: { code: "problem.wpCliDigest", detail: "the file downloaded is not WP-CLI 2.12.0" },
    }).api,
  },
  play: async ({ canvasElement }) => {
    const ui = await toTheTest(canvasElement);
    await userEvent.click(await ui.findByRole("button", { name: "Install WP-CLI…" }));
    await userEvent.click(await within(canvasElement).findByRole("button", { name: "Install WP-CLI" }));
    const dialog = within(await within(canvasElement).findByRole("dialog", { name: "Installing WP-CLI" }));
    await expect(await dialog.findByText("The file downloaded is refused")).toBeInTheDocument();
    await expect(dialog.getByRole("list", { name: "Steps of the install" })).toHaveTextContent("Digest check");
    await expect(dialog.getByRole("img", { name: "failed" })).toBeInTheDocument();
    await userEvent.click(dialog.getByRole("button", { name: "Close" }));
    // Still missing: the offer stays.
    await expect(ui.getByText("WP-CLI is missing on this account")).toBeInTheDocument();
  },
};

// No offer when PHP is too old for WP-CLI, nor over a ~/bin/wp that is there.
export const NoOffer: Story = {
  args: {
    api: fake([
      { phpVersion: "7.1.33", wpCli: "", wpCliVersion: "", install: null, problem: { code: "problem.wpCliPhpTooOld", detail: "7.1.33" } },
      { phpVersion: "8.4.27", wpCli: "", wpCliVersion: "", install: null, problem: { code: "problem.wpCliExists", detail: "~/bin/wp" } },
    ]).api,
  },
  play: async ({ canvasElement }) => {
    const ui = await toTheTest(canvasElement);
    await expect(await ui.findByText("PHP is too old for WP-CLI")).toBeInTheDocument();
    await expect(ui.getByRole("img", { name: "To check" })).toBeInTheDocument();
    await expect(ui.queryByRole("button", { name: "Install WP-CLI…" })).toBeNull();
    await userEvent.click(ui.getByRole("button", { name: "Try again" }));
    await expect(await ui.findByText("~/bin/wp already exists")).toBeInTheDocument();
    await expect(ui.getByRole("list", { name: "Checks" })).toHaveTextContent("~/bin/wp exists but does not answer as WP-CLI");
    await expect(ui.queryByRole("button", { name: "Install WP-CLI…" })).toBeNull();
  },
};
