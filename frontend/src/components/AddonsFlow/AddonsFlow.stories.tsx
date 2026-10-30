import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor, within } from "storybook/test";

import { addonsView } from "../../fixtures/addons";
import { vigiePreview } from "../../fixtures/data";
import { type AddonsApi, AddonsFlow } from "./AddonsFlow";

/** A fake Addons service: the file dialog answers a list, a run its preview. */
const api = (calls: (name: string, ...args: unknown[]) => void): AddonsApi => ({
  List: async () => addonsView,
  ChooseFile: async () => ({ file: "/Users/someone/servers.xml", problem: null }),
  Run: async (name, values) => {
    calls("Run", name, values);
    return { ...vigiePreview, source: "addon", addOn: name, addOnTitle: "Site list", report: "2 hosts." };
  },
});

const calls = fn();

const meta = {
  component: AddonsFlow,
  args: { api: api(calls), onLink: fn(), onPreview: fn() },
} satisfies Meta<typeof AddonsFlow>;
export default meta;

// From the grid to an add-on's page, its file chosen, its run handing over the preview.
export const RunASource: StoryObj<typeof meta> = {
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await userEvent.click(await page.findByRole("tab", { name: "Installed" }));
    await userEvent.click(page.getByRole("button", { name: "Open Site list" }));
    await userEvent.click(page.getByRole("button", { name: "Other file…" }));
    await waitFor(() => expect(page.getByRole("textbox", { name: "List" })).toHaveValue("/Users/someone/servers.xml"));
    await userEvent.click(page.getByRole("button", { name: "Run the import" }));
    await waitFor(() => expect(args.onPreview).toHaveBeenCalled());
    await expect(calls).toHaveBeenCalledWith("Run", "sitelist", { file: "/Users/someone/servers.xml", all: "false" });
  },
};
