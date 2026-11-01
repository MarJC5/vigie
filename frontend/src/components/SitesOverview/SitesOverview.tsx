import { useEffect, useId, useState } from "react";
import { useTranslation } from "react-i18next";

import { findingText } from "../../lib/findings";
import type { ExtensionsAcross, Problem, SiteResultView, SiteRow } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { BulkSummary } from "../BulkSummary/BulkSummary";
import { Button } from "../Button/Button";
import { Counts } from "../Counts/Counts";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { ExtensionsOverview } from "../ExtensionsOverview/ExtensionsOverview";
import { NotesPanel } from "../NotesPanel/NotesPanel";
import { SiteTable } from "../SiteTable/SiteTable";
import { TabBar } from "../TabBar/TabBar";
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
  /** The host's or folder's Markdown notes, under its table; without them, no notes. */
  notes?: string;
  /** Saves the notes; without it, they are only read. */
  onSaveNotes?: (notes: string) => Promise<Problem | null>;
  /** Opens a web address of the notes in the browser. */
  onOpen?: (href: string) => void;
  /** The plugins and themes across these sites; without it, no Plugins tab (ADR-0024). */
  extensions?: () => Promise<ExtensionsAcross>;
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
  notes,
  onSaveNotes,
  onOpen,
  extensions,
}: SitesOverviewProps) {
  const { t, i18n } = useTranslation();
  const pending = rows.reduce((n, r) => n + r.pending, 0);
  const count = (status: string) => rows.filter((r) => r.status === status).length;
  // A status chosen in the counts shows only its sites (ADR-0023, point 5).
  const [filter, setFilter] = useState<string>();
  const shown = filter === undefined ? rows : rows.filter((r) => r.status === filter);
  const [tab, setTab] = useState<"sites" | "extensions">("sites");
  const panel = useId();
  // The list across the sites, read when its tab opens.
  const [across, setAcross] = useState<ExtensionsAcross>();
  useEffect(() => {
    if (tab !== "extensions" || !extensions) return;
    let current = true;
    void extensions().then((a) => current && setAcross(a));
    return () => {
      current = false;
    };
  }, [tab, extensions]);
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
    <div className={styles.page}>
      <div className={styles.header}>
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
        {extensions && (
          <TabBar
            label={t("host.tabs.label")}
            tabs={[
              { id: "sites", label: t("host.tabs.sites") },
              { id: "extensions", label: t("tabs.extensions") },
            ]}
            value={tab}
            onChange={(id) => setTab(id as "sites" | "extensions")}
            panelId={panel}
          />
        )}
      </div>
      <div role={extensions ? "tabpanel" : undefined} id={panel} aria-labelledby={extensions ? `${panel}-${tab}` : undefined} className={styles.panel}>
        {tab === "extensions" && extensions ? (
          across && <ExtensionsOverview across={across} onSelect={onSelect} onOpen={onOpen} />
        ) : (
          <>
            <Counts
              label={t("host.summary")}
              items={[
                { label: t("host.sites"), value: rows.length },
                { label: t("host.urgent"), value: count("danger"), tone: "danger", filter: "danger" },
                { label: t("host.attention"), value: count("warn"), tone: "warn", filter: "warn" },
                { label: t("host.partial"), value: count("partial"), filter: "partial" },
                { label: t("host.ok"), value: count("ok"), filter: "ok" },
                ...(count("") > 0 ? [{ label: t("host.notScanned"), value: count(""), filter: "" }] : []),
              ]}
              selected={filter}
              onSelect={rows.length > 1 ? setFilter : undefined}
            >
              {filter !== undefined && (
                <Button variant="text" onClick={() => setFilter(undefined)}>
                  {t("host.showAll")}
                </Button>
              )}
            </Counts>
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
                rows={shown.map((r) => ({
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
            {notes !== undefined && <NotesPanel notes={notes} name={title} title={t("notes.title")} onSave={onSaveNotes} onOpen={onOpen} />}
          </>
        )}
      </div>
    </div>
  );
}
