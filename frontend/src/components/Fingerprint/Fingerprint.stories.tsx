import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Fingerprint } from "./Fingerprint";

const meta = {
  component: Fingerprint,
  args: {
    label: "Key of ssh.example-host.ch",
    type: "ED25519",
    value: "SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg",
    others: [
      { type: "ECDSA", value: "SHA256:+91qk9uB/U6iayJDcd19LyfUoBfNK///ByvoJTlXrbQ" },
      { type: "RSA", value: "SHA256:C7A6jy0J8YO6e3wDBgqIOPDXyuyIe3qo7twDzAIDY2c" },
    ],
    othersLabel: "The server also presents:",
    note: "Vigie keeps it in its own known_hosts; your SSH files are not changed.",
  },
} satisfies Meta<typeof Fingerprint>;
export default meta;

export const ServerKey: StoryObj<typeof meta> = {
  play: async ({ canvas }) => {
    const [region] = canvas.getAllByRole("group", { name: "Key of ssh.example-host.ch" });
    await expect(region).toHaveTextContent("SHA256:AKG7lYcQ7GZjus73uQl64gwWFRJRYApWtKiZEr5f5Cg");
    await expect(region).toHaveTextContent("ECDSA");
    await expect(canvas.getAllByText(/its own known_hosts/)).toHaveLength(2);
  },
};
