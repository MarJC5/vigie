import { useTranslation } from "react-i18next";

import type { SiteResultView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { type BulkOutcome, bulkCounts } from "../../lib/updates";
import { type Count, Counts } from "../Counts/Counts";
import { Icon, type IconName } from "../Icon/Icon";
import { Notice } from "../Notice/Notice";
import styles from "./BulkSummary.module.css";

export interface BulkSummaryProps {
  results: SiteResultView[];
  /** Opens a site, to roll it back or keep it. */
  onOpen: (siteID: string) => void;
}

const icons: Record<BulkOutcome, IconName> = {
  updated: "ok",
  failed: "failed",
  refused: "lock",
  nothing: "ok",
  "not-started": "circle",
  cancelled: "circle",
};

/** How the update of a host or folder ended: counts, the site it stopped at, and each site's
 result. */
export function BulkSummary({ results, onOpen }: BulkSummaryProps) {
  const { t } = useTranslation();
  const counts = bulkCounts(results);
  const items: Count[] = [
    { label: t("update.bulk.count.updated"), value: counts.updated },
    { label: t("update.bulk.count.failed"), value: counts.failed, tone: "warn" },
    { label: t("update.bulk.count.not-started"), value: counts["not-started"] },
  ];
  for (const outcome of ["refused", "nothing", "cancelled"] as const) {
    if (counts[outcome] > 0) items.push({ label: t(`update.bulk.count.${outcome}`), value: counts[outcome] });
  }
  const failed = results.filter((r) => r.outcome === "failed");

  return (
    <div className={styles.summary}>
      {failed.map((r) => (
        <Notice
          key={r.siteId}
          title={t("update.bulk.stoppedTitle", { site: siteName(r) })}
          text={t("update.bulk.stoppedText")}
          detail={r.detail || undefined}
          tone="warn"
          action={{ label: t("update.bulk.open", { site: siteName(r) }), onClick: () => onOpen(r.siteId) }}
        />
      ))}
      <Counts label={t("update.bulk.summary")} items={items} />
      <table aria-label={t("update.bulk.table")} className={styles.table}>
        <thead>
          <tr>
            <th scope="col">{t("update.bulk.col.site")}</th>
            <th scope="col">{t("update.bulk.col.result")}</th>
            <th scope="col">{t("update.bulk.col.detail")}</th>
          </tr>
        </thead>
        <tbody>
          {results.map((r) => {
            const outcome = r.outcome as BulkOutcome;
            return (
              <tr key={r.siteId} data-outcome={outcome}>
                <td className={styles.site} title={r.path}>
                  {siteName(r)}
                </td>
                <td>
                  <span className={styles.result}>
                    <span className={styles.icon}>
                      <Icon name={icons[outcome] ?? "circle"} />
                    </span>
                    {t(`update.bulk.outcome.${outcome}`)}
                  </span>
                </td>
                <td className={styles.detail} data-selectable>
                  {detailOf(t, r)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

type T = (key: string, vars?: Record<string, unknown>) => string;

function siteName(r: SiteResultView): string {
  return r.path.split("/").at(-1) ?? r.path;
}

/** Why a site failed or was refused, or why it was not started. */
function detailOf(t: T, r: SiteResultView): string {
  switch (r.outcome) {
    case "failed":
      return r.detail;
    case "refused":
      return r.detail.startsWith("problem.") || r.detail.startsWith("failure.") ? t(r.detail, { message: "" }) : r.detail;
    case "not-started":
      return t("update.bulk.notStarted");
  }
  return "";
}
