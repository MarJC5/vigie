import {
  ArchiveIcon,
  ArrowBendDownRightIcon,
  ArrowCircleUpIcon,
  ArrowCounterClockwiseIcon,
  ArrowsClockwiseIcon,
  ArrowSquareOutIcon,
  CaretDownIcon,
  CaretRightIcon,
  CheckCircleIcon,
  CheckIcon,
  CircleIcon,
  CircleNotchIcon,
  ClockCounterClockwiseIcon,
  CopyIcon,
  DatabaseIcon,
  DotsThreeIcon,
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
  QuestionIcon,
  ShieldCheckIcon,
  SidebarSimpleIcon,
  TerminalWindowIcon,
  TrashIcon,
  WarningCircleIcon,
  XCircleIcon,
  XIcon,
} from "@phosphor-icons/react";

// The only door to the icon set: names and sizes are decided here, always in the regular weight.
const icons = {
  activity: ClockCounterClockwiseIcon,
  archive: ArchiveIcon,
  caretDown: CaretDownIcon,
  caretRight: CaretRightIcon,
  check: CheckIcon,
  circle: CircleIcon,
  close: XIcon,
  copy: CopyIcon,
  database: DatabaseIcon,
  dots: DotsThreeIcon,
  external: ArrowSquareOutIcon,
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
