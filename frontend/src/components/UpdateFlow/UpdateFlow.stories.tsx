import type { Meta, StoryObj } from "@storybook/react-vite";
import { useMemo, useState } from "react";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import type { OperationView, UpdatesState } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { outdatedSite } from "../../fixtures/scans";
import { completed, failed, preview, rolledBack, running } from "../../fixtures/updates";
import { namesOf } from "../../lib/updates";
import { UpdateFlow, type UpdatesApi } from "./UpdateFlow";

const idle: UpdatesState = { running: [], active: 0, bulk: [], completed: 0, sites: [] };
const site = { id: "outdated", name: "boulangerie-exemple.ch" };
const keys = new Set(["plugin:akismet", "plugin:woocommerce", "languages"]);

/** A fake Updates service: a run shows its operation running, then ends as given and joins the
 history; refused, when set, answers every call. */
function Flow({ refused, follow, calls }: { refused?: boolean; follow?: boolean; calls: (name: string, ...args: unknown[]) => void }) {
  const [state, setState] = useState<UpdatesState>(follow ? { ...idle, running: [running], active: 1, sites: [site.id] } : idle);
  const [history, setHistory] = useState<OperationView[]>(follow ? [running, completed] : [completed]);
  const api = useMemo<UpdatesApi>(() => {
    // Runs an operation: running for a moment, then ended and stored.
    const play = (op: OperationView, end: OperationView) => {
      setState((s) => ({ ...s, running: [op], active: 1, sites: [site.id] }));
      setTimeout(() => {
        setHistory((h) => [end, ...h.filter((o) => o.id !== end.id)]);
        setState((s) => ({ ...s, running: [], active: 0, sites: [], completed: s.completed + 1 }));
      }, 300);
    };
    return {
      Preview: async (nodeId) => (calls("Preview", nodeId), [preview]),
      Update: async (siteId, sel) => {
        calls("Update", siteId, sel);
        if (refused) return { code: "problem.busy", detail: "" };
        play(running, failed);
        return null;
      },
      UpdateAll: async () => null,
      PreviewMany: async () => [preview],
      UpdateMany: async () => null,
      Rollback: async (siteId, opId) => {
        calls("Rollback", siteId, opId);
        play({ ...rolledBack, outcome: "running", finished: "", steps: rolledBack.steps!.slice(0, 1) }, rolledBack);
        return null;
      },
      CancelAll: async () => void calls("CancelAll"),
      History: async () => history,
    };
  }, [refused, calls, history]);
  return <UpdateFlow api={api} site={site} keys={keys} names={namesOf(outdatedSite.scan!.updates!)} state={state} follow={follow} onClose={fn()} />;
}

const meta = {
  component: Flow,
  args: { calls: fn() },
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
} satisfies Meta<typeof Flow>;
export default meta;
type Story = StoryObj<typeof meta>;

const first = <T,>(list: T[]) => list[0]!;

export const UpdateFailAndRollBack: Story = {
  play: async ({ canvas, args }) => {
    const confirm = first(await canvas.findAllByRole("button", { name: "Back up and update" }));
    await waitFor(() => expect(confirm).toBeEnabled());
    await userEvent.click(confirm);
    // Only what the preview offers is sent.
    await expect(args.calls).toHaveBeenCalledWith("Update", "outdated", {
      core: false,
      plugins: ["akismet", "woocommerce"],
      themes: [],
      languages: true,
    });
    await canvas.findAllByRole("heading", { name: "Updating boulangerie-exemple.ch" });
    await canvas.findAllByRole("heading", { name: "The update broke the site" }, { timeout: 3000 });
    await userEvent.click(first(canvas.getAllByRole("button", { name: "Roll back" })));
    await expect(args.calls).toHaveBeenCalledWith("Rollback", "outdated", "op-failed");
    await canvas.findAllByRole("heading", { name: "boulangerie-exemple.ch is rolled back" }, { timeout: 3000 });
  },
};

export const Refused: Story = {
  args: { refused: true },
  play: async ({ canvas }) => {
    const confirm = first(await canvas.findAllByRole("button", { name: "Back up and update" }));
    await waitFor(() => expect(confirm).toBeEnabled());
    await userEvent.click(confirm);
    const alert = first(await canvas.findAllByRole("alert"));
    await expect(alert).toHaveTextContent("An update or a rollback already runs on this site");
    await expect(canvas.getAllByRole("heading", { level: 1 })[0]).toHaveTextContent("Update boulangerie-exemple.ch");
  },
};

export const FollowARun: Story = {
  args: { follow: true },
  play: async ({ canvas, args }) => {
    await canvas.findAllByRole("heading", { name: "Updating boulangerie-exemple.ch" });
    await expect(args.calls).not.toHaveBeenCalledWith("Preview", "outdated");
    await userEvent.click(first(canvas.getAllByRole("button", { name: "Stop after this item" })));
    await expect(args.calls).toHaveBeenCalledWith("CancelAll");
    await expect(first(canvas.getAllByRole("button", { name: "Stopping after this item…" }))).toBeDisabled();
  },
};
