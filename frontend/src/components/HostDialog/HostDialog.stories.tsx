import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import type { HostDetails } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { HostDialog, type HostEditApi } from "./HostDialog";

const stored: HostDetails = {
  name: "Shared hosting", alias: "", hostname: "ssh.example-host.ch", port: 22, user: "u12ab", keyPath: "",
  wpCli: "wp-cli", folderId: "clients", maxConcurrent: 0,
};

// A fake of the Go services: a new host name clears the key, then the server presents one.
function fakeApi(): HostEditApi {
  return {
    Host: fn(async () => stored),
    TestConnection: fn(async () => ({ phpVersion: "8.4.27", wpCli: "wp", wpCliVersion: "2.12.0", install: null, problem: null })),
    EditHost: fn(async (_id: string, d: HostDetails) => {
      if (d.name === "") return { keyCleared: false, problem: { code: "problem.emptyName", detail: "" } };
      return { keyCleared: d.hostname !== stored.hostname, problem: null };
    }),
    HostKeys: fn(async () => ({
      name: "ssh2.example-host.ch",
      keys: [{ type: "ssh-ed25519", fingerprint: "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg" }],
      problem: null,
    })),
    Trust: fn(async () => null),
    KeepKey: fn(async () => null),
  };
}

const meta = {
  component: HostDialog,
  args: { api: fakeApi(), hostId: "shared", sites: 5, onClose: fn(), onSaved: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 720 }}><Story /></div>],
} satisfies Meta<typeof HostDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

const opened = async (canvas: HTMLElement) =>
  waitFor(() => within(canvas).getAllByRole("dialog", { name: "Edit the connection" })[1]!);

// A new command keeps the key: saved at once.
export const SaveTheCommand: Story = {
  args: { api: fakeApi() },
  play: async ({ canvasElement, args }) => {
    const dialog = await opened(canvasElement);
    await expect(dialog).toHaveAccessibleDescription("Shared hosting and its 5 sites use these settings.");
    const command = within(dialog).getByRole("textbox", { name: "WP-CLI command" });
    await userEvent.clear(command);
    await userEvent.type(command, "wp");
    await expect(within(dialog).queryByRole("status")).toBeNull();
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(args.onSaved).toHaveBeenCalled());
    await expect(args.api.HostKeys).not.toHaveBeenCalled();
  },
};

// A new host name: the dialog says so, then shows the server's key to check before trusting it.
export const NewAddressChecksTheKey: Story = {
  args: { api: fakeApi() },
  play: async ({ canvasElement, args }) => {
    const dialog = await opened(canvasElement);
    const hostname = within(dialog).getByRole("textbox", { name: "Host name" });
    await userEvent.clear(hostname);
    await userEvent.type(hostname, "ssh2.example-host.ch");
    await expect(within(dialog).getByRole("status")).toHaveTextContent("Vigie asks ssh2.example-host.ch for its key");
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    const key = await waitFor(() => within(canvasElement).getByRole("dialog", { name: "Check the key of ssh2.example-host.ch" }));
    const trust = within(key).getByRole("button", { name: "Trust" });
    await expect(trust).toBeDisabled();
    await userEvent.click(within(key).getByRole("checkbox"));
    await userEvent.click(trust);
    await waitFor(() => expect(args.onSaved).toHaveBeenCalled());
    await expect(args.api.KeepKey).toHaveBeenCalledWith("shared", "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg");
  },
};

// Problems go under their field.
export const Problems: Story = {
  args: { api: fakeApi() },
  play: async ({ canvasElement, args }) => {
    const dialog = await opened(canvasElement);
    await userEvent.clear(within(dialog).getByRole("textbox", { name: "Name in Vigie" }));
    const max = within(dialog).getByRole("textbox", { name: "Commands at a time" });
    await userEvent.type(max, "0");
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await expect(max).toHaveAccessibleDescription("A whole number from 1 to 10, or empty for 2.");
    await userEvent.clear(max);
    await userEvent.click(within(dialog).getByRole("button", { name: "Save" }));
    await waitFor(() => expect(within(dialog).getByRole("textbox", { name: "Name in Vigie" })).toHaveAccessibleDescription("The name cannot be empty."));
    await expect(args.onSaved).not.toHaveBeenCalled();
  },
};

// The connection as it is in the form can be tested before saving it.
export const TestBeforeSaving: Story = {
  args: { api: fakeApi(), onTest: fn() },
  play: async ({ canvasElement, args }) => {
    const dialog = await opened(canvasElement);
    const name = within(dialog).getByRole("textbox", { name: "Name in Vigie" });
    await userEvent.clear(name);
    await userEvent.type(name, "Shared hosting 2");
    await userEvent.click(within(dialog).getByRole("button", { name: "Test the connection" }));
    await expect(args.onTest).toHaveBeenCalledWith(expect.objectContaining({ name: "Shared hosting 2", hostname: stored.hostname }));
  },
};
