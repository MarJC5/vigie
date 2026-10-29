import { useTranslation } from "react-i18next";

import type { SiteRow } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { Counts } from "../Counts/Counts";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { SiteTable } from "../SiteTable/SiteTable";

export interface SitesOverviewProps {
  title: string;
  icon: "host" | "folder";
  /** The connection of a host. */
  subtitle?: string;
  rows: SiteRow[];
  scanLabel: string;
  scanning: boolean;
  onScan: () => void;
  onSelect: (siteId: string) => void;
}

/** A host or a folder: counts over its sites, then the table of their latest scans. */
export function SitesOverview({ title, icon, subtitle, rows, scanLabel, scanning, onScan, onSelect }: SitesOverviewProps) {
  const { t } = useTranslation();
  return (
    <>
      <DetailHeader title={title} icon={icon} subtitle={subtitle} />
      <Counts
        label={t("host.summary")}
        items={[
          { label: t("host.sites"), value: rows.length },
          { label: t("host.pending"), value: rows.reduce((n, r) => n + r.pending, 0), tone: "warn" },
          { label: t("host.attention"), value: rows.filter((r) => r.status === "warn").length, tone: "warn" },
          { label: t("host.notScanned"), value: rows.filter((r) => r.status === "").length },
        ]}
      >
        <Button variant="primary" icon="refresh" onClick={onScan} disabled={scanning} aria-busy={scanning || undefined}>
          {scanning ? t("overview.scanning") : scanLabel}
        </Button>
      </Counts>
      <SiteTable
        label={t("host.table", { name: title })}
        columns={{
          site: t("host.col.site"),
          status: t("host.col.status"),
          wordpress: t("host.col.wordpress"),
          php: t("host.col.php"),
          updates: t("host.col.updates"),
        }}
        notScanned={t("detail.notScanned")}
        rows={rows.map((r) => ({ id: r.id, name: r.name, status: r.status, wordpress: r.wordpress, php: r.phpCli, pending: r.pending }))}
        onSelect={onSelect}
      />
    </>
  );
}
