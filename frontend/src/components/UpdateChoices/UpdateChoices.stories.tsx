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
