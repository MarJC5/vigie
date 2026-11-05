import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent } from "storybook/test";

import { outdatedSite } from "../../fixtures/scans";
import { autoDone, autoRolledBack, autoRollbackFailed, autoRunning, autoStopped, completed, failed, preview, rolledBack, running } from "../../fixtures/updates";
import { namesOf, plannedItems, selectionOf } from "../../lib/updates";
import { UpdateWindow } from "./UpdateWindow";

const planned = plannedItems(preview, selectionOf(["plugin:akismet", "plugin:woocommerce", "languages"]));

const meta = {
  component: UpdateWindow,
  args: {
    name: "boulangerie-exemple.ch",
    preview,
    planned,
    names: namesOf(outdatedSite.scan!.updates!),
    operation: null,
    onClose: fn(),
    onConfirm: fn(),
    onStop: fn(),
    onRollback: fn(),
  },
  parameters: {
    layout: "fullscreen",
    // Rendered once per theme: the main landmark repeats by construction.
    a11y: { config: { rules: [{ id: "landmark-unique", enabled: false }, { id: "landmark-no-duplicate-main", enabled: false }] } },
  },
  decorators: [
    (Story) => (
      <div style={{ height: 760 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof UpdateWindow>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Loading: Story = {
  args: { preview: null },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("progressbar", { name: "Vigie reads on the server what will change." })).toHaveLength(2);
    await expect(canvas.getAllByRole("button", { name: "Back up and update" })[0]).toBeDisabled();
  },
};

export const Preview: Story = {
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("Update boulangerie-exemple.ch");
    await userEvent.click(canvas.getAllByRole("button", { name: "Back up and update" })[0]!);
    await expect(args.onConfirm).toHaveBeenCalled();
    await userEvent.click(canvas.getAllByRole("button", { name: "Cancel" })[0]!);
    await expect(args.onClose).toHaveBeenCalled();
  },
};

export const Busy: Story = {
  args: { problem: { code: "problem.busy", detail: "" } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("alert")[0]).toHaveTextContent("An update or a rollback already runs on this site");
  },
};

export const Starting: Story = {
  args: { starting: true },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Stop after this item" })[0]).toBeDisabled();
    // Every planned item waits.
    await expect(canvas.getAllByRole("img", { name: "Not run" })).toHaveLength(6);
    const [bar] = canvas.getAllByRole("progressbar", { name: "0 of 3 items updated and checked" });
    await expect(bar).toHaveAttribute("aria-valuemax", "3");
  },
};

export const Running: Story = {
  args: { operation: running },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("Updating boulangerie-exemple.ch");
    await expect(canvas.getAllByRole("button", { name: "Updating…" })[0]).toBeDisabled();
    await expect(canvas.getAllByRole("button", { name: "Updating…" })[0]).toHaveAttribute("aria-busy", "true");
    // Akismet is checked; WooCommerce is under way.
    await expect(canvas.getAllByRole("progressbar", { name: "1 of 3 items updated and checked" })[0]).toHaveAttribute("aria-valuenow", "1");
    await userEvent.click(canvas.getAllByRole("button", { name: "Stop after this item" })[0]!);
    await expect(args.onStop).toHaveBeenCalled();
  },
};

// Every step over, the run is not: Vigie scans the site again, and the last row says so.
export const Rescanning: Story = {
  args: { operation: { ...completed, outcome: "running", finished: "" }, planned: planned.filter((it) => it.kind === "plugin" && it.slug === "akismet") },
  play: async ({ canvas }) => {
    const [list] = canvas.getAllByRole("list", { name: "Steps of the update" });
    await expect(list!.lastElementChild).toHaveTextContent("New scan of the site");
    await expect(list!.lastElementChild).toHaveAttribute("aria-current", "step");
    await expect(canvas.getAllByRole("button", { name: "Stop after this item" })[0]).toBeDisabled();
    // The translations count once their last step is over: here only Akismet was planned.
    await expect(canvas.getAllByRole("progressbar", { name: "1 of 1 items updated and checked" })).toHaveLength(2);
  },
};

export const Stopping: Story = {
  args: { operation: running, stopping: true },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("button", { name: "Stopping after this item…" })[0]).toBeDisabled();
  },
};

export const Failed: Story = {
  args: { operation: failed },
  play: async ({ canvas, args }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("WooCommerce: the site no longer passes the checks");
    await expect(alert).toHaveTextContent("Roll back puts the folder of WooCommerce back as it was");
    await expect(alert).toHaveTextContent("wc_get_page_id");
    await userEvent.click(canvas.getAllByRole("button", { name: "Roll back" })[0]!);
    await expect(args.onRollback).toHaveBeenCalledWith("op-failed");
    await userEvent.click(canvas.getAllByRole("button", { name: "Keep as is" })[0]!);
    await expect(args.onClose).toHaveBeenCalled();
  },
};

export const RollingBack: Story = {
  args: { operation: { ...rolledBack, outcome: "running", finished: "", steps: rolledBack.steps!.slice(0, 1) } },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("Rolling back boulangerie-exemple.ch");
    await expect(canvas.queryAllByRole("button", { name: "Stop after this item" })).toHaveLength(0);
    await expect(canvas.getAllByRole("progressbar", { name: "Rolling back…" })).toHaveLength(2);
  },
};

export const RolledBack: Story = {
  args: { operation: rolledBack },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("boulangerie-exemple.ch is rolled back");
    await expect(canvas.getAllByText("WooCommerce is back to its earlier version, and the site passes the checks again.")).toHaveLength(2);
  },
};

export const Done: Story = {
  args: { operation: completed },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("boulangerie-exemple.ch is up to date");
    await userEvent.click(canvas.getAllByRole("button", { name: "Close" }).at(-1)!);
    await expect(args.onClose).toHaveBeenCalled();
  },
};

// ADR-0042: WooCommerce broke the site and went back on its own; the run goes on.
export const RolledBackOnItsOwn: Story = {
  args: { operation: autoRunning, rollbacks: [autoRolledBack] },
  play: async ({ canvas }) => {
    const [note] = canvas.getAllByRole("note");
    await expect(note).toHaveTextContent("WooCommerce 10.2.0 broke the site: back to 9.8.1 on its own");
    await expect(note).toHaveTextContent("The update goes on with the next items.");
    const [list] = canvas.getAllByRole("list", { name: "Steps of the update" });
    await expect(list).toHaveTextContent("WooCommerce back to 9.8.1");
    await expect(list).toHaveTextContent("on its own, after the failure above");
    await expect(canvas.getAllByRole("progressbar", { name: "1 of 3 items updated and checked, 1 rolled back" })[0]).toHaveAttribute("aria-valuenow", "2");
  },
};

export const DoneButOneRolledBack: Story = {
  args: { operation: autoDone, rollbacks: [autoRolledBack] },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("boulangerie-exemple.ch is up to date, but WooCommerce");
    const [note] = canvas.getAllByRole("note");
    await expect(note).toHaveTextContent("Version 10.2.0 is set aside");
    await expect(note).toHaveTextContent("wc_get_page_id");
    await expect(canvas.queryAllByRole("button", { name: "Roll back" })).toHaveLength(0);
  },
};

export const RollbackOnItsOwnFailed: Story = {
  args: { operation: autoStopped, rollbacks: [autoRollbackFailed] },
  play: async ({ canvas, args }) => {
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("The rollback failed");
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("WooCommerce is back to 9.8.1, but the site still fails its checks");
    await expect(alert).toHaveTextContent("status 500");
    // The rollback stays offered, to try again.
    await userEvent.click(canvas.getAllByRole("button", { name: "Roll back" })[0]!);
    await expect(args.onRollback).toHaveBeenCalledWith("op-auto");
  },
};

// WordPress never goes back on its own: its rollback stays offered, and says why.
export const WordPressFailed: Story = {
  args: { operation: { ...failed, rollback: { kind: "core", slug: "", language: "", from: "7.1.3", to: "7.1.4", ignored: false, setAside: false } } },
  play: async ({ canvas }) => {
    const [alert] = canvas.getAllByRole("alert");
    await expect(alert).toHaveTextContent("Vigie never does it on its own");
    await expect(canvas.getAllByRole("button", { name: "Roll back" })).toHaveLength(2);
  },
};
