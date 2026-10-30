import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { ConsentScreen } from "./ConsentScreen";

// Each story renders once per theme, so the main landmark repeats by construction; the end-to-end
// test checks that the window has one.
const meta = {
  component: ConsentScreen,
  parameters: {
    a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }, { id: "landmark-no-duplicate-main", enabled: false }] } },
  },
  args: { destinations: ["wpvulnerability", "wordpress.org", "registry", "site"], onAgree: fn(), onNotNow: fn() },
  decorators: [(Story) => <div style={{ height: 720 }}><Story /></div>],
} satisfies Meta<typeof ConsentScreen>;
export default meta;
type Story = StoryObj<typeof meta>;

// Every service listed with what it receives; IANA comes with the registries, without a switch.
export const Agree: Story = {
  play: async ({ canvasElement, args }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(screen.getAllByRole("switch")).toHaveLength(4);
    await expect(screen.getByRole("cell", { name: "IANA" })).toBeInTheDocument();
    await userEvent.click(screen.getByRole("switch", { name: "Domain registry (.com, .fr…)" }));
    await expect(screen.getByRole("switch", { name: "Domain registry (.com, .fr…)" })).toHaveAttribute("aria-checked", "false");
    await userEvent.click(screen.getByRole("button", { name: "Agree" }));
    await expect(args.onAgree).toHaveBeenCalledWith(["wpvulnerability", "wordpress.org", "site"]);
  },
};

export const NotNow: Story = {
  play: async ({ canvasElement, args }) => {
    const screen = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await userEvent.click(screen.getByRole("button", { name: "Not now" }));
    await expect(args.onNotNow).toHaveBeenCalled();
    await expect(args.onAgree).not.toHaveBeenCalled();
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { name: "Avant le premier relevé de sécurité" })).toHaveLength(2);
    await expect(canvas.getAllByRole("button", { name: "Accepter" })).toHaveLength(2);
  },
};
