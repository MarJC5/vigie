import type { Meta, StoryObj } from "@storybook/react-vite";
import { expect } from "storybook/test";

import { Icon, type IconProps, iconNames, iconSizes } from "./Icon";
import { VigieIcon } from "./VigieIcon";

const meta = { component: Icon, args: { name: "folder" } } satisfies Meta<typeof Icon>;
export default meta;
type Story = StoryObj<typeof meta>;

export const Decorative: Story = {
  play: async ({ canvasElement }) => {
    const svg = canvasElement.querySelector("svg");
    await expect(svg).toHaveAttribute("aria-hidden", "true");
    await expect(svg).toHaveAttribute("width", "16");
  },
};

export const Labelled: Story = {
  args: { name: "warning", label: "Needs attention", size: "nav" },
  play: async ({ canvas }) => {
    await expect(canvas.getAllByRole("img", { name: "Needs attention" })).toHaveLength(2);
  },
};

// Six sizes, each with its role (ADR-0004): xs 12, sm 14, md 16, nav 22, lg 28, xl 40.
export const Sizes: Story = {
  render: () => (
    <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
      {(Object.keys(iconSizes) as (keyof typeof iconSizes)[]).map((size) => (
        <Icon key={size} name="shield" size={size} />
      ))}
    </div>
  ),
  play: async ({ canvasElement }) => {
    const widths = [...canvasElement.querySelectorAll("[data-theme=dark] svg")].map((svg) => svg.getAttribute("width"));
    await expect(widths).toEqual(["12", "14", "16", "22", "28", "40"]);
  },
};

// A size is one of the six names, never a number of pixels: tsc fails if a number comes back.
// @ts-expect-error a number of pixels is refused
const refused: IconProps = { name: "shield", size: 18 };
void refused;

export const Gallery: Story = {
  render: () => (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 12 }}>
      {iconNames.map((name) => (
        <Icon key={name} name={name} size="nav" />
      ))}
    </div>
  ),
};

// Vigie's own glyph, in Phosphor's six weights; the window uses the regular one.
export const VigieWeights: Story = {
  render: () => (
    <div style={{ display: "flex", gap: 16 }}>
      {(["thin", "light", "regular", "bold", "fill", "duotone"] as const).map((weight) => (
        <VigieIcon key={weight} weight={weight} size={48} aria-hidden="true" />
      ))}
    </div>
  ),
  play: async ({ canvasElement }) => {
    await expect(canvasElement.querySelectorAll("svg")).toHaveLength(12); // six weights, two themes
  },
};
