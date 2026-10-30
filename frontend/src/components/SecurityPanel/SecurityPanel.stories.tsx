import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, within } from "storybook/test";

import { exploited, findings, hidden } from "../../fixtures/findings";
import { outdatedSite } from "../../fixtures/scans";
import { SecurityPanel } from "./SecurityPanel";

const scan = {
  ...outdatedSite.scan!,
  status: "danger",
  filesChecked: "2026-10-30T08:57:00Z",
  findings,
  counts: { urgent: 2, attention: 5, notChecked: 1, ignored: 1 },
};

const meta = {
  component: SecurityPanel,
  args: { scan, onCheckFiles: fn(), onIgnore: fn(), onRestore: fn(), onOpen: fn() },
} satisfies Meta<typeof SecurityPanel>;
export default meta;
type Story = StoryObj<typeof meta>;

// Grouped the worst first, each finding with its shape and words; the ignored one last, dimmed.
export const Findings: Story = {
  play: async ({ canvasElement, args }) => {
    const panel = within(canvasElement.querySelector<HTMLElement>("[data-theme=dark]")!);
    await expect(panel.getByText("2 urgent · 5 need attention · 1 not checked · 1 ignored")).toBeInTheDocument();
    const groups = panel.getAllByRole("heading", { level: 2 }).map((h) => h.textContent);
    await expect(groups).toEqual(["Urgent2", "Needs attention5", "Not checked1", "Information2", "Ignored1"]);
    await expect(panel.getByText("WooCommerce 9.8.1: SQL injection without authentication")).toBeInTheDocument();
    await expect(panel.getByText("CVSS 9.8, critical · exploited (KEV) · fixed in 9.8.3")).toBeInTheDocument();

    await userEvent.click(panel.getByRole("button", { name: "Ignore “WooCommerce 9.8.1: SQL injection without authentication”" }));
    await expect(args.onIgnore).toHaveBeenCalledWith(exploited);
    await userEvent.click(panel.getByRole("button", { name: "Stop ignoring “Hidden from search engines”" }));
    await expect(args.onRestore).toHaveBeenCalledWith(hidden);
    // Information is only shown: nothing to ignore.
    await expect(panel.queryByRole("button", { name: /Ignore “TLS certificate/ })).toBeNull();

    await userEvent.click(panel.getByRole("link", { name: /CVE-2026-41234/ }));
    await expect(args.onOpen).toHaveBeenCalledWith("https://www.cve.org/CVERecord?id=CVE-2026-41234");
    await userEvent.click(panel.getByRole("button", { name: "Check files now" }));
    await expect(args.onCheckFiles).toHaveBeenCalled();
  },
};

export const InFrench: Story = {
  globals: { lang: "fr" },
  play: async ({ canvasElement }) => {
    const panel = within(canvasElement.querySelector<HTMLElement>("[data-theme=light]")!);
    await expect(panel.getByText("2 urgents · 5 à surveiller · 1 non vérifié · 1 ignoré")).toBeInTheDocument();
    await expect(panel.getByText(/Fichiers vérifiés le 30\.10\.2026/)).toBeInTheDocument();
  },
};

export const CheckingFiles: Story = {
  args: { scanning: true },
  play: async ({ canvas }) => {
    const [button] = canvas.getAllByRole("button", { name: "Checking files…" });
    await expect(button).toBeDisabled();
  },
};

export const NothingFound: Story = {
  args: { scan: { ...scan, findings: [], counts: { urgent: 0, attention: 0, notChecked: 0, ignored: 0 }, filesChecked: "" } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByText("Nothing to report")).toHaveLength(2);
    await expect(canvas.getAllByText("Files not compared yet")).toHaveLength(2);
  },
};
