import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect, fn, userEvent, waitFor } from "storybook/test";

import { AdminAddressDialog } from "./AdminAddressDialog";

const computed = "https://boulangerie-exemple.ch/wp-admin/";

const meta = {
  component: AdminAddressDialog,
  args: { own: "", computed, onSave: fn(async () => null), onClose: fn() },
  decorators: [
    (Story) => (
      <div style={{ position: "relative", height: 420 }}>
        <Story />
      </div>
    ),
  ],
} satisfies Meta<typeof AdminAddressDialog>;
export default meta;
type Story = StoryObj<typeof meta>;

// The computed address to start from; the one typed is saved.
export const RenamedLogin: Story = {
  play: async ({ canvas, args }) => {
    const [field] = canvas.getAllByRole("textbox", { name: "Address" });
    await expect(field).toHaveValue(computed);
    await expect(canvas.getAllByText(`Computed: ${computed}`).length).toBeGreaterThan(0);
    await expect(canvas.queryAllByRole("button", { name: "Back to the computed address" })).toHaveLength(0);
    await userEvent.clear(field!);
    await userEvent.type(field!, "https://boulangerie-exemple.ch/connexion");
    const [save] = canvas.getAllByRole("button", { name: "Save" });
    await userEvent.click(save!);
    await expect(args.onSave).toHaveBeenCalledWith("https://boulangerie-exemple.ch/connexion");
  },
};

// An address already given can go back to the computed one; a refused one says why.
export const AddressGiven: Story = {
  args: {
    own: "https://boulangerie-exemple.ch/connexion",
    onSave: fn(async (address: string) => (address === "" ? null : { code: "problem.invalidAddress", detail: "" })),
  },
  play: async ({ canvas, args }) => {
    const [field] = canvas.getAllByRole("textbox", { name: "Address" });
    await expect(field).toHaveValue("https://boulangerie-exemple.ch/connexion");
    const [save] = canvas.getAllByRole("button", { name: "Save" });
    await userEvent.click(save!);
    await waitFor(() => expect(canvas.getAllByText("Give a web address, starting with https:// (or http://).").length).toBeGreaterThan(0));
    const [reset] = canvas.getAllByRole("button", { name: "Back to the computed address" });
    await userEvent.click(reset!);
    await expect(args.onSave).toHaveBeenCalledWith("");
  },
};
