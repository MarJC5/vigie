import type { CopyPreviewView, CopyRunView, CopyView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

// The copies of boulangerie-exemple.ch, as the mockups of row 28 show them (ADR-0041).

export const copiesFolder = "~/Documents/Vigie/Shared hosting/boulangerie-exemple.ch";

export const preview: CopyPreviewView = {
  siteId: "site",
  database: 12_400_000,
  content: 2_224_000_000,
  site: 2_274_000_000,
  folders: [
    { name: "uploads", bytes: 1_840_000_000 },
    { name: "backups-dup-lite", bytes: 1_020_000_000 },
    { name: "plugins", bytes: 312_000_000 },
    { name: "cache", bytes: 96_300_000 },
    { name: "themes", bytes: 41_200_000 },
    { name: "", bytes: 18_400_000 },
  ],
  split: true,
  destination: copiesFolder,
  free: 214_000_000_000,
  archiveSize: 500,
};

const names = ["database.sql.gz", "plugins.tar.gz", "themes.tar.gz", "uploads.tar.gz", "uploads-2.tar.gz", "uploads-3.tar.gz", "uploads-4.tar.gz", "others.tar.gz"];
const sizes = [3_100_000, 288_400_000, 38_900_000, 497_600_000, 499_100_000, 498_400_000, 341_000_000, 17_200_000];

const parts = (written: (i: number) => boolean) => names.map((name, i) => ({ name, bytes: sizes[i]!, written: written(i) }));

export const complete: CopyView = {
  folder: `${copiesFolder}/2026-11-03 20.41`,
  started: "2026-11-03T19:41:00Z",
  content: "content",
  leftOut: ["cache", "backups-dup-lite"],
  archives: 8,
  planned: 8,
  bytes: 2_183_700_000,
  complete: true,
  parts: parts(() => true),
};

export const database: CopyView = {
  folder: `${copiesFolder}/2026-10-28 09.15`,
  started: "2026-10-28T08:15:00Z",
  content: "database",
  leftOut: [],
  archives: 1,
  planned: 1,
  bytes: 3_000_000,
  complete: true,
  parts: [{ name: "database.sql.gz", bytes: 3_000_000, written: true }],
};

export const incomplete: CopyView = {
  folder: `${copiesFolder}/2026-10-15 14.03`,
  started: "2026-10-15T12:03:00Z",
  content: "content",
  leftOut: [],
  archives: 6,
  planned: 8,
  bytes: 1_336_800_000,
  complete: false,
  parts: parts((i) => i !== 5 && i !== 6),
};

// A copy running: its first five archives written, the sixth being read.
export const runningCopy: CopyView = { ...complete, archives: 5, complete: false, bytes: 1_327_100_000, parts: parts((i) => i < 5) };

export const run: CopyRunView = {
  siteId: "site",
  folder: "",
  archive: "uploads-3.tar.gz",
  done: 5,
  total: 8,
  read: 212_000_000,
  expected: 498_000_000,
  resumed: false,
};
