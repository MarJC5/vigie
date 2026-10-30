import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { WizardStep } from "./WizardStep";

// Each story renders once per theme, so the main landmark repeats by construction; the end-to-end
// test of the wizard checks that the window has one.
const twoThemes = {
  a11y: {
    config: {
      rules: [
        { id: "landmark-unique", enabled: false },
        { id: "landmark-no-duplicate-main", enabled: false },
      ],
    },
  },
};

const meta = {
  component: WizardStep,
  args: {
    title: "Test the connection",
    lead: "Vigie checks the connection, PHP on the command line and WP-CLI. Nothing is written on the server.",
    steps: ["Connection", "Server key", "Test", "Sites"],
    current: 2,
    stepsLabel: "Steps",
    doneLabel: "done",
    closeLabel: "Close",
    onClose: fn(),
    back: { label: "Go back", onClick: fn() },
    primary: { label: "Search for sites", onClick: fn() },
    children: <p>Checks</p>,
  },
  parameters: { layout: "fullscreen", ...twoThemes },
  decorators: [
    (Story) => (
      <div style={{ height: 760 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof WizardStep>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Ready: Story = {
  play: async ({ canvas, args }) => {
    const [title] = canvas.getAllByRole("heading", { level: 1, name: "Test the connection" });
    await expect(title).toBeInTheDocument();
    const [primary] = canvas.getAllByRole("button", { name: "Search for sites" });
    await userEvent.click(primary!);
    await expect(args.primary.onClick).toHaveBeenCalled();
    const [back] = canvas.getAllByRole("button", { name: "Go back" });
    await userEvent.click(back!);
    await expect(args.back!.onClick).toHaveBeenCalled();
    const [close] = canvas.getAllByRole("button", { name: "Close" });
    await userEvent.click(close!);
    await expect(args.onClose).toHaveBeenCalled();
  },
};

export const NotReady: Story = {
  args: { current: 0, back: undefined, primary: { label: "Continue", onClick: fn(), disabled: true } },
  play: async ({ canvas }) => {
    const [primary] = canvas.getAllByRole("button", { name: "Continue" });
    await expect(primary).toBeDisabled();
    await expect(canvas.queryAllByRole("button", { name: "Go back" })).toHaveLength(0);
  },
};

export const Busy: Story = {
  args: { primary: { label: "Search for sites", onClick: fn(), busy: true } },
  play: async ({ canvas }) => {
    const [primary] = canvas.getAllByRole("button", { name: "Search for sites" });
    await expect(primary).toHaveAttribute("aria-busy", "true");
    await expect(primary).toBeDisabled();
  },
};

// A wait: the button turns, the bar runs along the whole bottom of the window, its sentence stays
// with the content.
export const Waiting: Story = {
  args: {
    primary: { label: "Search for sites", onClick: fn(), busy: true },
    progress: { label: "Checking the installs found: 2 of 6", value: 2, max: 6 },
  },
  play: async ({ canvas, canvasElement }) => {
    const [primary] = canvas.getAllByRole("button", { name: "Search for sites" });
    await expect(primary!.querySelector("svg")).not.toBeNull();
    const [bar] = canvas.getAllByRole("progressbar", { name: "Checking the installs found: 2 of 6" });
    await expect(bar).toHaveAttribute("aria-valuenow", "2");
    // As wide as the window.
    const window = canvasElement.querySelector("[data-theme] > div > div")!.getBoundingClientRect();
    await expect(Math.round(bar!.getBoundingClientRect().width)).toBe(Math.round(window.width));
  },
};
