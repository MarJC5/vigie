import type { Meta, StoryObj } from "@storybook/react-vite";
import { useState } from "react";
import { expect, fn, userEvent } from "storybook/test";

import { outdatedSite } from "../../fixtures/scans";
import { choicesOf, namesOf, offeredKeys } from "../../lib/updates";
import { UpdateChoices } from "./UpdateChoices";

const updates = outdatedSite.scan!.updates!;
const groups = choicesOf(updates);

const meta = {
  component: UpdateChoices,
  args: { groups, chosen: offeredKeys(groups), onChange: fn(), names: namesOf(updates) },
  render: function Render(args) {
    const [chosen, setChosen] = useState(args.chosen);
    return (
      <UpdateChoices
        {...args}
        chosen={chosen}
        onChange={(next) => {
          setChosen(next);
          args.onChange(next);
        }}
      />
    );
  },
} satisfies Meta<typeof UpdateChoices>;
export default meta;

export const Choices: StoryObj<typeof meta> = {
  play: async ({ canvas, args }) => {
    const [woo] = canvas.getAllByRole("checkbox", { name: /WooCommerce/ });
    await expect(woo).toBeChecked();
    await userEvent.click(woo!);
    await expect(args.onChange).toHaveBeenLastCalledWith(new Set(["core", "plugin:akismet", "languages"]));
    // A plugin not offered has no checkbox, and says why.
    await expect(canvas.queryAllByRole("checkbox", { name: /Old Gallery/ })).toHaveLength(0);
    await expect(canvas.getAllByText("Not offered: needs a newer PHP or WordPress")).toHaveLength(2);
    // The translations are one choice, naming what they translate.
    await expect(canvas.getAllByRole("checkbox", { name: "fr_FR: WordPress and Akismet Anti-spam" })).toHaveLength(2);
    await expect(canvas.getAllByText("All up to date")).toHaveLength(2);
  },
};

// An update ignored (ADR-0026): apart, unticked, with why; each offered update has its changelog
// and "Ignore".
const ignore = { untilVersion: "", reason: "payment not tested", created: "2026-11-01T10:00:00Z" };
const held = choicesOf(updates.map((u) => (u.slug === "woocommerce" ? { ...u, ignore } : u)));

export const Ignored: StoryObj<typeof meta> = {
  args: { groups: held, chosen: offeredKeys(held), onIgnore: fn(), onRestore: fn(), onOpen: fn() },
  play: async ({ canvas, args }) => {
    const [group] = canvas.getAllByRole("group", { name: /^Ignored/ });
    await expect(group).toHaveTextContent("Ignored permanently on 01.11.2026: “payment not tested”");
    const [woo] = canvas.getAllByRole("checkbox", { name: /WooCommerce/ });
    await expect(woo).not.toBeChecked();
    await userEvent.click(canvas.getAllByRole("button", { name: "Stop ignoring the update of WooCommerce" })[0]!);
    await expect(args.onRestore).toHaveBeenCalledWith(expect.objectContaining({ key: "plugin:woocommerce" }));
    await userEvent.click(canvas.getAllByRole("button", { name: "Ignore the update of Akismet Anti-spam" })[0]!);
    await expect(args.onIgnore).toHaveBeenCalledWith(expect.objectContaining({ ignoreKey: "update|plugin:akismet|" }));
    await userEvent.click(canvas.getAllByRole("link", { name: /Release notes/ })[0]!);
    await expect(args.onOpen).toHaveBeenCalledWith("https://wordpress.org/documentation/wordpress-version/version-7-1-4/");
    // Translations cannot be ignored on their own; a plugin not offered neither.
    await expect(canvas.queryAllByRole("button", { name: /Ignore the update of (fr_FR|Old Gallery)/ })).toHaveLength(0);
  },
};

// A version gone back on its own (ADR-0042): set aside in a group of its own, unticked, with when;
// ticking it updates it on purpose.
const setAside = { created: "2026-11-04T14:12:24Z", reason: "http://boulangerie-exemple.ch/ answers 500" };
const aside = choicesOf(updates.map((u) => (u.slug === "woocommerce" ? { ...u, setAside } : u)));

export const SetAside: StoryObj<typeof meta> = {
  args: { groups: aside, chosen: offeredKeys(aside) },
  play: async ({ canvas, args }) => {
    const [group] = canvas.getAllByRole("group", { name: /^Set aside/ });
    await expect(group).toHaveTextContent("Rolled back on its own on 04.11.2026. Left out until a newer version.");
    const [woo] = canvas.getAllByRole("checkbox", { name: /WooCommerce/ });
    await expect(woo).not.toBeChecked();
    await userEvent.click(woo!);
    await expect(args.onChange).toHaveBeenLastCalledWith(new Set(["core", "plugin:akismet", "languages", "plugin:woocommerce"]));
  },
};
