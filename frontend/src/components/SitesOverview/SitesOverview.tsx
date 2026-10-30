import { useTranslation } from "react-i18next";

import { findingText } from "../../lib/findings";
import type { SiteResultView, SiteRow } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { BulkSummary } from "../BulkSummary/BulkSummary";
import { Button } from "../Button/Button";
import { Counts } from "../Counts/Counts";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { SiteTable } from "../SiteTable/SiteTable";
import styles from "./SitesOverview.module.css";

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
  /** Opens the host's or folder's menu under the point given. */
  onMore?: (point: { x: number; y: number }) => void;
  /** Opens the menu of one of the sites, at the point of a right-click on its row. */
  onSiteMenu?: (siteId: string, point: { x: number; y: number }) => void;
  /** Asks to update every site; without it, no such button. */
  onUpdateAll?: () => void;
  /** A site here is in an update not over yet. */
  updating?: boolean;
  /** How the last update of every site here ended, shown in place of the table until dismissed. */
  bulk?: SiteResultView[];
  onDismissBulk?: () => void;
}

/** A host or a folder: counts over its sites, then the table of their latest scans, or how their
 update ended. */
export function SitesOverview({
  title,
  icon,
  subtitle,
  rows,
  scanLabel,
  scanning,
  onScan,
  onSelect,
  onMore,
  onSiteMenu,
  onUpdateAll,
  updating = false,
  bulk,
  onDismissBulk,
}: SitesOverviewProps) {
  const { t, i18n } = useTranslation();
  const pending = rows.reduce((n, r) => n + r.pending, 0);
  const count = (status: string) => rows.filter((r) => r.status === status).length;
  if (bulk && bulk.length > 0) {
    return (
      <>
        <DetailHeader
          title={title}
          icon={icon}
          subtitle={subtitle}
          actions={
            onDismissBulk && (
              <Button variant="outline" onClick={onDismissBulk}>
                {t("update.bulk.dismiss")}
              </Button>
            )
          }
        />
        <BulkSummary results={bulk} onOpen={onSelect} />
      </>
    );
  }
  return (
    <>
      <DetailHeader
        title={title}
        icon={icon}
        subtitle={subtitle}
        actions={
          <>
            {onUpdateAll && (
              <Button icon="update" onClick={onUpdateAll} disabled={updating || scanning || pending === 0} aria-busy={updating || undefined}>
                {updating ? t("update.updatingAll") : t("update.updateAll")}
              </Button>
            )}
            <Button variant="primary" icon="refresh" onClick={onScan} disabled={scanning || updating} aria-busy={scanning || undefined}>
              {scanning ? t("overview.scanning") : scanLabel}
            </Button>
          </>
        }
      >
        {onMore && (
          <Button
            variant="icon"
            icon="dots"
            label={t("edit.more")}
            onClick={(event) => {
              const r = event.currentTarget.getBoundingClientRect();
              onMore({ x: r.left, y: r.bottom + 4 });
            }}
          />
        )}
      </DetailHeader>
      <Counts
        label={t("host.summary")}
        items={[
          { label: t("host.sites"), value: rows.length },
          { label: t("host.urgent"), value: count("danger"), tone: "danger" },
          { label: t("host.attention"), value: count("warn"), tone: "warn" },
          { label: t("host.partial"), value: count("partial") },
          { label: t("host.ok"), value: count("ok") },
          ...(count("") > 0 ? [{ label: t("host.notScanned"), value: count("") }] : []),
        ]}
      />
      {rows.length === 0 ? (
        <p className={styles.empty}>{t(icon === "host" ? "edit.emptyHost" : "edit.emptyFolder")}</p>
      ) : (
        <SiteTable
          label={t("host.table", { name: title })}
          columns={{
            site: t("host.col.site"),
            status: t("host.col.status"),
            worst: t("host.col.worst"),
            updates: t("host.col.updates"),
          }}
          notScanned={t("detail.notScanned")}
          rows={rows.map((r) => ({
            id: r.id,
            name: r.name,
            status: r.status,
            worst: r.worst ? findingText(r.worst, t, i18n.language).title : "",
            pending: r.pending,
          }))}
          onSelect={onSelect}
          onMenu={onSiteMenu}
        />
      )}
    </>
  );
}
