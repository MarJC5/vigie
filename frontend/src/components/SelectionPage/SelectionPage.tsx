import { useTranslation } from "react-i18next";

import type { SiteResultView, SiteRow } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { BulkSummary } from "../BulkSummary/BulkSummary";
import { Button } from "../Button/Button";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { SitesList } from "../SitesOverview/SitesOverview";
import styles from "./SelectionPage.module.css";

export interface SelectionPageProps {
  /** "3 items selected". */
  title: string;
  /** What the selection holds: "2 hosts, 1 site · 6 sites in all". */
  subtitle: string;
  /** Every site the selection holds, each once. */
  rows: SiteRow[];
  scanLabel: string;
  scanning: boolean;
  onScan: () => void;
  /** A site here is in an update not over yet. */
  updating?: boolean;
  onUpdateAll?: () => void;
  /** Moves the folders and hosts; without it, none is selected. */
  onMove?: () => void;
  /** Opens the colors under the point given. */
  onColor?: (point: { x: number; y: number }) => void;
  onTags?: () => void;
  /** Copies the sites to this computer (ADR-0041, point 8). */
  onCopy?: () => void;
  onDelete?: () => void;
  onSelect: (siteId: string) => void;
  onSiteMenu?: (siteId: string, point: { x: number; y: number }) => void;
  /** How the last update of these sites ended, shown in place of the table until dismissed. */
  bulk?: SiteResultView[];
  onDismissBulk?: () => void;
}

/** Several items selected in the site list (ADR-0034): what they hold, the actions on all of them,
 and the table of their sites. */
export function SelectionPage({
  title,
  subtitle,
  rows,
  scanLabel,
  scanning,
  onScan,
  updating = false,
  onUpdateAll,
  onMove,
  onColor,
  onTags,
  onCopy,
  onDelete,
  onSelect,
  onSiteMenu,
  bulk,
  onDismissBulk,
}: SelectionPageProps) {
  const { t } = useTranslation();
  const pending = rows.reduce((n, r) => n + r.pending, 0);
  if (bulk && bulk.length > 0) {
    return (
      <>
        <DetailHeader
          title={title}
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
      <DetailHeader title={title} subtitle={subtitle} />
      <div className={styles.actions}>
        <Button variant="primary" icon="refresh" onClick={onScan} disabled={scanning || updating || rows.length === 0} aria-busy={scanning || undefined}>
          {scanning ? t("overview.scanning") : scanLabel}
        </Button>
        {onUpdateAll && (
          <Button icon="update" onClick={onUpdateAll} disabled={updating || scanning || pending === 0} aria-busy={updating || undefined}>
            {updating ? t("update.updatingAll") : t("update.updateAll")}
          </Button>
        )}
        {onMove && (
          <Button icon="move" onClick={onMove}>
            {t("edit.menu.move")}
          </Button>
        )}
        {onColor && (
          <Button
            icon="palette"
            aria-haspopup="menu"
            onClick={(event) => {
              const r = event.currentTarget.getBoundingClientRect();
              onColor({ x: r.left, y: r.bottom + 4 });
            }}
          >
            {t("edit.menu.color")}
          </Button>
        )}
        {onTags && rows.length > 0 && (
          <Button icon="tag" onClick={onTags}>
            {t("selection.action.tags")}
          </Button>
        )}
        {onCopy && rows.length > 0 && (
          <Button icon="archive" onClick={onCopy}>
            {t("selection.action.copy")}
          </Button>
        )}
        {onDelete && (
          <Button icon="trash" onClick={onDelete}>
            {t("selection.action.delete")}
          </Button>
        )}
      </div>
      <SitesList label={t("selection.table")} rows={rows} where empty={t("selection.empty")} onSelect={onSelect} onSiteMenu={onSiteMenu} />
      <p className={styles.hint}>{t("selection.hint")}</p>
    </div>
  );
}
