// A site's backups on its server, as the latest scan read them (ADR-0033), for the stories.
import type { BackupsView, SiteBackupsView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

/** The two updates of fixtures/updates, and an older one past 30 days. */
export const backups: BackupsView = {
  read: true,
  days: 30,
  kept: 3,
  items: [
    { at: "2026-09-18T07:40:00Z", bytes: 45_100_000, expired: true, byAge: true },
    { at: "2026-10-22T09:03:10Z", bytes: 47_900_000, expired: false, byAge: false },
    { at: "2026-10-29T14:12:03Z", bytes: 48_200_000, expired: false, byAge: false },
  ],
};

/** The sites of the host "Shared hosting", with what they keep on its server. */
export const hostBackups: SiteBackupsView[] = [
  { siteId: "atelier", read: true, count: 1, bytes: 38_000_000 },
  { siteId: "boulangerie", read: true, count: 3, bytes: 141_200_000 },
  { siteId: "ecoles", read: true, count: 1, bytes: 31_800_000 },
];
