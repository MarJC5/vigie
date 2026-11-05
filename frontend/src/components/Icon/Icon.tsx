import {
  ArchiveIcon,
  ArrowBendDownRightIcon,
  ArrowCircleUpIcon,
  ArrowCounterClockwiseIcon,
  ArrowsClockwiseIcon,
  ArrowSquareOutIcon,
  BellIcon,
  CaretDownIcon,
  CaretUpIcon,
  CaretRightIcon,
  CheckCircleIcon,
  CheckIcon,
  CircleIcon,
  CircleNotchIcon,
  ClockCounterClockwiseIcon,
  ClockIcon,
  CopyIcon,
  DatabaseIcon,
  DotsThreeIcon,
  EyeSlashIcon,
  FolderIcon,
  FolderOpenIcon,
  GlobeIcon,
  GearSixIcon,
  GlobeSimpleIcon,
  HardDrivesIcon,
  type Icon as PhosphorIcon,
  ListMagnifyingGlassIcon,
  LockIcon,
  MagnifyingGlassIcon,
  NotePencilIcon,
  PaletteIcon,
  PencilSimpleIcon,
  PlusIcon,
  PuzzlePieceIcon,
  QuestionIcon,
  ShieldCheckIcon,
  SidebarSimpleIcon,
  TagIcon,
  TerminalWindowIcon,
  TrashIcon,
  WarningCircleIcon,
  XCircleIcon,
  XIcon,
} from "@phosphor-icons/react";

import { VigieIcon } from "./VigieIcon";

// The only door to the icon set: names and sizes are decided here, always in the regular weight.
const icons = {
  activity: ClockCounterClockwiseIcon,
  archive: ArchiveIcon,
  bell: BellIcon,
  caretDown: CaretDownIcon,
  caretUp: CaretUpIcon,
  caretRight: CaretRightIcon,
  check: CheckIcon,
  circle: CircleIcon,
  clock: ClockIcon,
  close: XIcon,
  copy: CopyIcon,
  database: DatabaseIcon,
  dots: DotsThreeIcon,
  external: ArrowSquareOutIcon,
  eyeSlash: EyeSlashIcon,
  failed: XCircleIcon,
  findSites: ListMagnifyingGlassIcon,
  folder: FolderIcon,
  folderOpen: FolderOpenIcon,
  globe: GlobeIcon,
  help: QuestionIcon,
  host: HardDrivesIcon,
  lock: LockIcon,
  move: ArrowBendDownRightIcon,
  note: NotePencilIcon,
  ok: CheckCircleIcon,
  palette: PaletteIcon,
  plus: PlusIcon,
  puzzle: PuzzlePieceIcon,
  refresh: ArrowsClockwiseIcon,
  rename: PencilSimpleIcon,
  rollback: ArrowCounterClockwiseIcon,
  search: MagnifyingGlassIcon,
  settings: GearSixIcon,
  shield: ShieldCheckIcon,
  sidebar: SidebarSimpleIcon,
  sites: GlobeSimpleIcon,
  spinner: CircleNotchIcon,
  tag: TagIcon,
  terminal: TerminalWindowIcon,
  trash: TrashIcon,
  update: ArrowCircleUpIcon,
  vigie: VigieIcon,
  warning: WarningCircleIcon,
} satisfies Record<string, PhosphorIcon>;

export type IconName = keyof typeof icons;
export const iconNames = Object.keys(icons) as IconName[];

/** Six sizes, each with its role (ADR-0004): xs carets, the cross of a tag, an icon in 12 px text;
 * sm beside 13 or 14 px text; md the start of a row, a menu, a Notice, a step; nav the navigation
 * rail; lg the header of a page or a dialog; xl an empty state, a full screen. */
export const iconSizes = { xs: 12, sm: 14, md: 16, nav: 22, lg: 28, xl: 40 } as const;
export type IconSize = keyof typeof iconSizes;

export interface IconProps {
  name: IconName;
  /** One of the six names, never a number of pixels. */
  size?: IconSize;
  /** When set, the icon carries meaning and is announced; otherwise it is decorative. */
  label?: string;
}

export function Icon({ name, size = "md", label }: IconProps) {
  const Glyph = icons[name];
  const px = iconSizes[size];
  return label ? (
    <Glyph size={px} weight="regular" role="img" aria-label={label} />
  ) : (
    <Glyph size={px} weight="regular" aria-hidden="true" />
  );
}
