import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import security from "../../../../docs/security.md?raw";
import securityFr from "../../../../docs/fr/security.md?raw";
import { headingId, Markdown, outline } from "./Markdown";

const meta = {
  component: Markdown,
  args: { text: security, onLink: fn() },
} satisfies Meta<typeof Markdown>;
export default meta;

type Story = StoryObj<typeof meta>;

export const SecurityDoc: Story = {
  play: async ({ canvas, args }) => {
    // The doc's title sits under the page's own: h2, then its sections in h3, with their ids.
    const [title] = canvas.getAllByRole("heading", { level: 2 });
    await expect(title).toHaveTextContent("Security scans: what Vigie reads, asks and sends");
    const [section] = canvas.getAllByRole("heading", { level: 3, name: "What is read on the server" });
    await expect(section).toHaveAttribute("id", "doc-what-is-read-on-the-server");
    await expect(outline(args.text).map((h) => h.id)).toContain(section!.id);
    // The table of what is sent, and a link that goes to onLink, never followed by the window.
    await expect(canvas.getAllByRole("table")[0]).toHaveTextContent("api.wordpress.org");
    const [link] = canvas.getAllByRole("link", { name: "WPVulnerability" });
    await userEvent.click(link!);
    await expect(args.onLink).toHaveBeenCalledWith("https://www.wpvulnerability.com/");
  },
};

export const SecurityDocInFrench: Story = {
  args: { text: securityFr },
  play: async ({ canvas }) => {
    const [section] = canvas.getAllByRole("heading", { level: 3, name: "Ce qui est lu sur le serveur" });
    await expect(section).toHaveAttribute("id", headingId("Ce qui est lu sur le serveur"));
    await expect(section).toHaveAttribute("id", "doc-ce-qui-est-lu-sur-le-serveur");
  },
};

export const RawHtmlIsText: Story = {
  args: {
    text: "# A doc\n\nSome <em>emphasis</em> written in HTML, and a [link to another doc](data.md).\n\n<div>A block of HTML</div>\n",
  },
  play: async ({ canvas, canvasElement, args }) => {
    // HTML in a doc is shown as it is written, never inserted in the window.
    await expect(canvas.getAllByText(/<em>emphasis<\/em>/).length).toBeGreaterThan(0);
    await expect(canvas.getAllByText("<div>A block of HTML</div>").length).toBeGreaterThan(0);
    await expect(canvasElement.querySelector("em")).toBeNull();
    await expect(canvasElement.querySelector("[data-selectable] div div")).toBeNull();
    const [link] = canvas.getAllByRole("link", { name: "link to another doc" });
    await userEvent.click(link!);
    await expect(args.onLink).toHaveBeenCalledWith("data.md");
  },
};
