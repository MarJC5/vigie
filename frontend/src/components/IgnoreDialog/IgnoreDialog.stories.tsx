import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { fileEditing, xss } from "../../fixtures/findings";
import { IgnoreDialog } from "./IgnoreDialog";

const meta = {
  component: IgnoreDialog,
  args: { finding: xss, site: "boulangerie-exemple.ch", onIgnore: fn(), onClose: fn() },
  decorators: [(Story) => <div style={{ position: "relative", height: 520 }}><Story /></div>],
} satisfies Meta<typeof IgnoreDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// A plugin's finding: until its next version by default, a reason optional.
export const UntilTheNextVersion: Story = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getByRole("button", { name: /Until the next version of Contact Form 7/ })).toHaveAttribute("aria-pressed", "true");
    await userEvent.click(dialog.getByRole("button", { name: "Ignore" }));
    await expect(args.onIgnore).toHaveBeenCalledWith("6.0.4", "");
  },
};

// Permanently: the button waits for a reason.
export const Permanently: Story = {
  play: async ({ canvasElement, args }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await userEvent.click(dialog.getByRole("button", { name: /Permanently/ }));
    const ignore = dialog.getByRole("button", { name: "Ignore" });
    await expect(ignore).toBeDisabled();
    await userEvent.type(dialog.getByRole("textbox", { name: "Reason" }), "  Form disabled until the end of the year ");
    await userEvent.click(ignore);
    await expect(args.onIgnore).toHaveBeenCalledWith("", "Form disabled until the end of the year");
  },
};

// A setting has no version: only "permanently".
export const ASetting: Story = {
  args: { finding: fileEditing },
  play: async ({ canvasElement }) => {
    const dialog = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark] [role=dialog]")!);
    await expect(dialog.getAllByRole("button").filter((b) => b.hasAttribute("aria-pressed"))).toHaveLength(1);
    await expect(dialog.getByRole("button", { name: /Permanently/ })).toHaveAttribute("aria-pressed", "true");
  },
};

export const Refused: Story = {
  args: { problem: { code: "problem.reasonRequired", detail: "" } },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("A reason is required to ignore a finding permanently.");
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    const [title] = canvas.getAllByRole("heading", { name: "Ignorer «\u00a0Contact Form 7 6.0.4\u00a0: Stored cross-site scripting\u00a0»\u00a0?" });
    await expect(title).toBeInTheDocument();
  },
};

// An update offered (ADR-0026): until a newer version by default, or permanently with a reason.
export const AnUpdate: Story = {
  args: { finding: undefined, update: { name: "WooCommerce", from: "9.8.1", to: "10.2.0" } },
  play: async ({ canvas, args }) => {
    const [dialog] = canvas.getAllByRole("dialog", { name: "Ignore the update of WooCommerce?" });
    await expect(dialog).toHaveAccessibleDescription("9.8.1 → 10.2.0. It no longer counts in the site's status, and “Update all” leaves it out.");
    await expect(canvas.getAllByText("Comes back by itself as soon as a version newer than 10.2.0 is offered.").length).toBeGreaterThan(0);
    await userEvent.click(canvas.getAllByRole("button", { name: "Ignore" })[0]!);
    await expect(args.onIgnore).toHaveBeenCalledWith("10.2.0", "");
  },
};
