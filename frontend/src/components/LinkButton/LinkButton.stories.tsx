import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { ExternalLink } from "../ExternalLink/ExternalLink";
import { LinkButton } from "./LinkButton";

const meta = {
  component: LinkButton,
  args: { children: "See the finding", onClick: fn() },
} satisfies Meta<typeof LinkButton>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Click: Story = {
  play: async ({ canvas, args }) => {
    const [button] = canvas.getAllByRole("button", { name: "See the finding" });
    await userEvent.click(button!);
    await expect(args.onClick).toHaveBeenCalledTimes(1);
  },
};

/** Enter and the space bar, as any button. */
export const Keyboard: Story = {
  play: async ({ canvas, args }) => {
    const [button] = canvas.getAllByRole("button", { name: "See the finding" });
    await userEvent.tab();
    await expect(button).toHaveFocus();
    await userEvent.keyboard("{Enter}");
    await userEvent.keyboard(" ");
    await expect(args.onClick).toHaveBeenCalledTimes(2);
  },
};

/** ExternalLink's skin: the same color, size and weight for the same tone. */
export const SameAsExternalLink: Story = {
  render: () => (
    <div style={{ display: "flex", gap: 24 }}>
      <LinkButton onClick={fn()}>See the finding</LinkButton>
      <ExternalLink href="https://wordpress.org/plugins/woocommerce/">Page on wordpress.org</ExternalLink>
    </div>
  ),
  play: async ({ canvas }) => {
    const [button] = canvas.getAllByRole("button", { name: "See the finding" });
    const [link] = canvas.getAllByRole("link", { name: /Page on wordpress.org/ });
    const a = getComputedStyle(button!);
    const b = getComputedStyle(link!);
    await expect(a.color).toBe(b.color);
    await expect(a.fontSize).toBe(b.fontSize);
    await expect(a.fontWeight).toBe(b.fontWeight);
    await expect(a.backgroundColor).toBe("rgba(0, 0, 0, 0)");
  },
};

export const Tones: Story = {
  render: () => (
    <div style={{ display: "grid", justifyItems: "start", gap: 14 }}>
      <LinkButton onClick={fn()}>See the finding</LinkButton>
      <LinkButton tone="text" size="body" onClick={fn()}>
        boulangerie-exemple.ch
      </LinkButton>
      <LinkButton tone="quiet" icon="terminal" onClick={fn()}>
        SSH terminal
      </LinkButton>
      <LinkButton icon="eyeSlash" onClick={fn()}>
        Ignore
      </LinkButton>
      <LinkButton disabled onClick={fn()}>
        Back
      </LinkButton>
      <p style={{ margin: 0 }}>
        A scan of{" "}
        <LinkButton size="inline" onClick={fn()}>
          atelier-exemple.ch
        </LinkButton>{" "}
        found 2 changes.
      </p>
    </div>
  ),
};

/** In running text: the text's size and weight, underlined so that it stands apart (WCAG 1.4.1). */
export const Inline: Story = {
  render: () => (
    <p style={{ margin: 0, fontSize: 14 }}>
      A scan of{" "}
      <LinkButton size="inline" onClick={fn()}>
        atelier-exemple.ch
      </LinkButton>{" "}
      found 2 changes.
    </p>
  ),
  play: async ({ canvas }) => {
    const [button] = canvas.getAllByRole("button", { name: "atelier-exemple.ch" });
    const style = getComputedStyle(button!);
    await expect(style.fontSize).toBe("14px");
    await expect(style.textDecorationLine).toBe("underline");
  },
};

/** The quiet tone: muted, the text's color on hover, never underlined. */
export const Quiet: Story = {
  args: { tone: "quiet", icon: "terminal", children: "SSH terminal" },
  play: async ({ canvas }) => {
    const [button] = canvas.getAllByRole("button", { name: "SSH terminal" });
    await userEvent.hover(button!);
    await expect(getComputedStyle(button!).textDecorationLine).toBe("none");
  },
};

export const Disabled: Story = {
  args: { disabled: true, children: "Back" },
  play: async ({ canvas, args }) => {
    const [button] = canvas.getAllByRole("button", { name: "Back" });
    await expect(button).toBeDisabled();
    await userEvent.click(button!);
    await expect(args.onClick).not.toHaveBeenCalled();
  },
};

/** A name read by screen readers when the text alone would not say enough. */
export const Labelled: Story = {
  args: { label: "Ignore “Contact Form 7 6.0.4: stored XSS”", icon: "eyeSlash", children: "Ignore" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Ignore “Contact Form 7 6.0.4: stored XSS”" })[0]).toBeInTheDocument();
  },
};
