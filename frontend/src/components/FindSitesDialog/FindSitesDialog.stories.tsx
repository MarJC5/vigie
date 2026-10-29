import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import type { DiscoverResult } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { type FindSitesApi, FindSitesDialog } from "./FindSitesDialog";

const found: DiscoverResult = {
  truncated: false,
  problem: null,
  installs: [
    { path: "~/sites/atelier", state: "already-added", name: "atelier-exemple.ch", url: "https://atelier-exemple.ch", multisite: false, subsites: 0, message: "" },
    { path: "~/sites/nouveau", state: "new", name: "nouveau-exemple.ch", url: "https://nouveau-exemple.ch", multisite: false, subsites: 0, message: "" },
    { path: "~/sites/essai", state: "new", name: "essai.example.test", url: "https://essai.example.test", multisite: false, subsites: 0, message: "" },
  ],
};

const api = (result: DiscoverResult): FindSitesApi => ({
  DiscoverHost: fn(async () => result),
  AddSites: fn(async () => null),
});

const meta = {
  component: FindSitesDialog,
  args: { api: api(found), hostId: "shared", hostName: "Shared hosting", onClose: fn(), onAdded: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 560 }}><Story /></div>],
} satisfies Meta<typeof FindSitesDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// New installs come checked; one already in Vigie cannot be chosen.
export const NewInstalls: Story = {
  args: { api: api(found) },
  play: async ({ canvas, args }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Find new sites" })[1]!;
    const add = await within(dialog).findByRole("button", { name: "Add 2 sites" });
    await userEvent.click(within(dialog).getByRole("checkbox", { name: "Add essai.example.test" }));
    await expect(within(dialog).getByRole("button", { name: "Add 1 site" })).toBeEnabled();
    await userEvent.click(add);
    await waitFor(() => expect(args.onAdded).toHaveBeenCalled());
    await expect(args.api.AddSites).toHaveBeenCalledWith("shared", [
      { path: "~/sites/nouveau", name: "nouveau-exemple.ch", url: "https://nouveau-exemple.ch", multisite: false },
    ]);
  },
};

export const NothingNew: Story = {
  args: { api: api({ ...found, installs: found.installs!.slice(0, 1) }) },
  play: async ({ canvas }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Find new sites" })[1]!;
    await expect(await within(dialog).findByText("No new install on this account.")).toBeInTheDocument();
    await expect(within(dialog).getByRole("button", { name: "Close" })).toBeInTheDocument();
  },
};

export const SearchFailed: Story = {
  args: { api: api({ installs: [], truncated: false, problem: { code: "problem.findFailed", detail: "find: missing argument to -maxdepth" } }) },
  play: async ({ canvas }) => {
    const dialog = canvas.getAllByRole("dialog", { name: "Find new sites" })[1]!;
    await expect(await within(dialog).findByRole("alert")).toHaveTextContent("The search for WordPress installs failed");
  },
};
