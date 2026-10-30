import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent, within } from "storybook/test";

import { cloudDestination, listSource } from "../../fixtures/addons";
import { AddonPage, type AddonPageProps } from "./AddonPage";

function Stateful(args: AddonPageProps) {
  const [values, setValues] = useState(args.values);
  return (
    <AddonPage
      {...args}
      values={values}
      onValue={(id, v) => {
        setValues((x) => ({ ...x, [id]: v }));
        args.onValue(id, v);
      }}
    />
  );
}

const meta = {
  component: AddonPage,
  args: {
    addon: listSource,
    values: { file: "", all: "false" },
    onValue: fn(),
    onChooseFile: fn(),
    onRun: fn(),
    onBack: fn(),
    onLink: fn(),
  },
  render: (args) => <Stateful {...args} />,
} satisfies Meta<typeof AddonPage>;
export default meta;
type Story = StoryObj<typeof meta>;

// A source: its settings, the run waits for the required one, the lock says what it can do.
export const ASource: Story = {
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("heading", { level: 1, name: "Site list" })).toBeInTheDocument();
    await expect(page.getByText("by Someone · Source · version 0.2.0")).toBeInTheDocument();
    const run = page.getByRole("button", { name: "Run the import" });
    await expect(run).toBeDisabled();
    await userEvent.click(page.getByRole("button", { name: "Other file…" }));
    await expect(args.onChooseFile).toHaveBeenCalledWith("file");
    await userEvent.type(page.getByRole("textbox", { name: "List" }), "/Users/someone/servers.xml");
    await userEvent.click(page.getByRole("checkbox", { name: /Every server/ }));
    await expect(args.onValue).toHaveBeenLastCalledWith("all", "true");
    await userEvent.click(run);
    await expect(args.onRun).toHaveBeenCalled();
    await expect(page.getByText("/opt/homebrew/bin/vigie-sitelist")).toBeInTheDocument();
    await userEvent.click(page.getByRole("link", { name: "Add-ons" }));
    await expect(args.onBack).toHaveBeenCalled();
  },
};

export const Running: Story = {
  args: { values: { file: "/Users/someone/servers.xml" }, running: true },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(page.getByRole("button", { name: "Running…" })).toBeDisabled();
  },
};

// A failure says what the add-on said, word for word.
export const Failed: Story = {
  args: {
    values: { file: "/Users/someone/servers.xml" },
    failure: { problem: { code: "problem.addonFailed", detail: "exit status 1" }, report: "vigie-sitelist: servers.xml: not a list" },
  },
  play: async ({ canvasElement }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByRole("alert")).toHaveTextContent("vigie-sitelist stopped with an error");
    await expect(page.getByText("vigie-sitelist: servers.xml: not a list")).toBeInTheDocument();
  },
};

export const ADestinationNotInstalled: Story = {
  args: { addon: cloudDestination },
  play: async ({ canvasElement, args }) => {
    const page = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(page.getByText("Not installed. Vigie never installs an add-on itself.")).toBeInTheDocument();
    await userEvent.click(page.getByRole("link", { name: /How to install/ }));
    await expect(args.onLink).toHaveBeenCalledWith("https://example.test/vigie-cloudsave");
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Lancer l'import" })).toHaveLength(2);
    await expect(canvas.getAllByText("par Someone · Source · version 0.2.0")).toHaveLength(2);
  },
};
