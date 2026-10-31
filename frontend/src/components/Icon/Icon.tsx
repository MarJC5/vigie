import {
  ArchiveIcon,
  ArrowBendDownRightIcon,
  ArrowCircleUpIcon,
  ArrowCounterClockwiseIcon,
  ArrowsClockwiseIcon,
  ArrowSquareOutIcon,
  BellIcon,
  CaretDownIcon,
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
  terminal: TerminalWindowIcon,
  trash: TrashIcon,
  update: ArrowCircleUpIcon,
  vigie: VigieIcon,
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
