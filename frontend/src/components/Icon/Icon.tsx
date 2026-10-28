import {
  ArrowsClockwiseIcon,
  ArrowSquareOutIcon,
  CaretDownIcon,
  CaretRightIcon,
  CheckCircleIcon,
  ClockCounterClockwiseIcon,
  CopyIcon,
  FolderIcon,
  FolderOpenIcon,
  GearSixIcon,
  GlobeSimpleIcon,
  HardDrivesIcon,
  type Icon as PhosphorIcon,
  MagnifyingGlassIcon,
  NotePencilIcon,
  PlusIcon,
  QuestionIcon,
  ShieldCheckIcon,
  SidebarSimpleIcon,
  TerminalWindowIcon,
  WarningCircleIcon,
} from "@phosphor-icons/react";

// The only door to the icon set: names, weights and sizes are decided here.
const icons = {
  activity: ClockCounterClockwiseIcon,
  caretDown: CaretDownIcon,
  caretRight: CaretRightIcon,
  copy: CopyIcon,
  external: ArrowSquareOutIcon,
  folder: FolderIcon,
  folderOpen: FolderOpenIcon,
  help: QuestionIcon,
  host: HardDrivesIcon,
  note: NotePencilIcon,
  ok: CheckCircleIcon,
  plus: PlusIcon,
  refresh: ArrowsClockwiseIcon,
  search: MagnifyingGlassIcon,
  settings: GearSixIcon,
  shield: ShieldCheckIcon,
  sidebar: SidebarSimpleIcon,
  sites: GlobeSimpleIcon,
  terminal: TerminalWindowIcon,
  warning: WarningCircleIcon,
} satisfies Record<string, PhosphorIcon>;

export type IconName = keyof typeof icons;
export const iconNames = Object.keys(icons) as IconName[];

export interface IconProps {
  name: IconName;
  /** Pixels: 16 in lists, 20 to 22 in the navigation rail. */
  size?: number;
  /** "fill" marks the active item of the navigation rail. */
  weight?: "regular" | "fill";
  /** When set, the icon carries meaning and is announced; otherwise it is decorative. */
  label?: string;
}

export function Icon({ name, size = 16, weight = "regular", label }: IconProps) {
  const Glyph = icons[name];
  return label ? (
    <Glyph size={size} weight={weight} role="img" aria-label={label} />
  ) : (
    <Glyph size={size} weight={weight} aria-hidden="true" />
  );
}
