import { useTranslation } from "react-i18next";

import type { BackupsView, OperationView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { backupRun, operationSummary } from "../../lib/updates";
import { Button } from "../Button/Button";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./BackupList.module.css";

export interface BackupListProps {
  backups: BackupsView;
  /** The site's updates, to name the one each backup was made before. */
  operations: OperationView[];
  names: Map<string, string>;
  /** Asks to remove the backups past the limits; without it, they are only listed. */
  onRemove?: () => void;
  /** An update, a rollback or a scan runs on the site: nothing can be removed. */
  busy?: boolean;
}

/** A site's backups on its server, as the latest scan read them (ADR-0033): each update's date, what
 it was, its size; those the limits remove are marked, and can be removed at once. */
export function BackupList({ backups, operations, names, onRemove, busy = false }: BackupListProps) {
  const { t, i18n } = useTranslation();
  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "short", timeStyle: "short" });
  const size = (bytes: number) => sizeText(bytes, i18n.language);
  const byRun = new Map(operations.filter((op) => op.backupDir).map((op) => [op.backupDir.slice(op.backupDir.lastIndexOf("/") + 1), op]));
  const items = backups.items ?? [];
  const expired = items.filter((b) => b.expired);
  const caption = !backups.read
    ? t("backups.notRead")
    : backups.days > 0
      ? t("backups.caption", { days: backups.days, kept: backups.kept })
      : t("backups.captionNoLimit", { kept: backups.kept });
  return (
    <div className={styles.backups}>
      <div className={styles.header}>
        <h2 className={styles.title}>{t("backups.title")}</h2>
        <span className={styles.caption}>{caption}</span>
      </div>
      {backups.read && items.length === 0 && <p className={styles.none}>{t("backups.none")}</p>}
      {items.length > 0 && (
        <div className={styles.list}>
          <ul className={styles.rows}>
            {items.map((b) => {
              const op = byRun.get(backupRun(b.at));
              return (
                <li key={b.at} className={styles.row}>
                  <span className={styles.when}>{when.format(new Date(b.at))}</span>
                  <span className={styles.what}>{op ? operationSummary(t, op, names) : t("backups.update")}</span>
                  <span className={styles.size}>{size(b.bytes)}</span>
                  <span className={styles.mark}>
                    {b.expired && (
                      <>
                        <StatusDot status="warn" />
                        {b.byAge ? t("backups.pastDays", { days: backups.days }) : t("backups.beyond", { kept: backups.kept })}
                      </>
                    )}
                  </span>
                </li>
              );
            })}
          </ul>
          {expired.length > 0 && (
            <div className={styles.footer}>
              <span className={styles.caption}>
                {t(expired.every((b) => b.byAge) ? "backups.oldByAge" : "backups.old", {
                  count: expired.length,
                  days: backups.days,
                  size: size(expired.reduce((n, b) => n + b.bytes, 0)),
                })}
              </span>
              {onRemove && (
                <Button icon="trash" onClick={onRemove} disabled={busy}>
                  {t("backups.removeAction")}
                </Button>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
