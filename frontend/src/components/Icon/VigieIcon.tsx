import { IconBase, type IconProps, type IconWeight } from "@phosphor-icons/react";
import { forwardRef, type ReactElement } from "react";

// Vigie's mark as a glyph of the set: the eye of the logo, its pupil a shield, in Phosphor's six
// weights. As in Phosphor, the outlines keep one center line on a 256 grid and only their width
// changes (thin 8, light 12, regular 16, bold 24); fill is the eye as wide as the regular one's
// outer edge, cut by the shield; duotone is the regular lines over the eye at 20 %.
const eye = "M128 56C48 56 16 128 16 128s32 72 112 72 112-72 112-72S208 56 128 56Z";
const outerEye = "M128 48C44 48 8 128 8 128s36 80 120 80 120-80 120-80S212 48 128 48Z";
const shield = "M94 98c22-6 46-6 68 0v28c0 24-14 38-34 46-20-8-34-22-34-46Z";

const lines = (width: number) => (
  <g fill="none" stroke="currentColor" strokeWidth={width} strokeLinecap="round" strokeLinejoin="round">
    <path d={eye} />
    <path d={shield} />
  </g>
);

const weights = new Map<IconWeight, ReactElement>([
  ["thin", lines(8)],
  ["light", lines(12)],
  ["regular", lines(16)],
  ["bold", lines(24)],
  ["fill", <path d={outerEye + shield} fill="currentColor" fillRule="evenodd" />],
  [
    "duotone",
    <>
      <path d={eye} fill="currentColor" opacity={0.2} />
      {lines(16)}
    </>,
  ],
]);

export const VigieIcon = forwardRef<SVGSVGElement, IconProps>((props, ref) => <IconBase ref={ref} {...props} weights={weights} />);
VigieIcon.displayName = "VigieIcon";
