import {
  ArrowsClockwiseIcon,
  ArrowSquareOutIcon,
  CaretDownIcon,
  CaretRightIcon,
  CheckIcon,
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
  XIcon,
} from "@phosphor-icons/react";

// The only door to the icon set: names and sizes are decided here, always in the regular weight.
const icons = {
  activity: ClockCounterClockwiseIcon,
  caretDown: CaretDownIcon,
  caretRight: CaretRightIcon,
  check: CheckIcon,
  close: XIcon,
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
  /** When set, the icon carries meaning and is announced; otherwise it is decorative. */
  label?: string;
}

export function Icon({ name, size = 16, label }: IconProps) {
  const Glyph = icons[name];
  return label ? (
    <Glyph size={size} weight="regular" role="img" aria-label={label} />
  ) : (
    <Glyph size={size} weight="regular" aria-hidden="true" />
  );
}
