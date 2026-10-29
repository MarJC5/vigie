import { useTranslation } from "react-i18next";

import type { SitePreviewView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { pendingOf } from "../../lib/updates";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import styles from "./BulkDialog.module.css";

export interface BulkDialogProps {
  /** The host or folder. */
  name: string;
  /** The previews of its sites; null while Vigie reads them. */
  previews: SitePreviewView[] | null;
  onConfirm: () => void;
  onClose: () => void;
}

/** Asks before updating every site of a host or folder: what each would change, or why it cannot. */
export function BulkDialog({ name, previews, onConfirm, onClose }: BulkDialogProps) {
  const { t } = useTranslation();
  const ready = (previews ?? []).filter((p) => !p.problem && p.dumpTool && pendingOf(p) > 0).length;

  return (
    <Dialog
      title={t("update.bulkDialog.title", { name })}
      lead={t("update.bulkDialog.lead")}
      onClose={onClose}
      wide
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button variant="primary" icon="update" onClick={onConfirm} disabled={ready === 0} aria-busy={!previews || undefined}>
            {t("update.bulkDialog.confirm", { count: ready })}
          </Button>
        </>
      }
    >
      {previews ? (
        <ul aria-label={t("update.bulkDialog.sites")} className={styles.sites}>
          {previews.map((p) => {
            const count = pendingOf(p);
            const why = p.problem
              ? t(p.problem.code, { message: p.problem.detail })
              : !p.dumpTool
                ? t("problem.noDumpTool")
                : count === 0
                  ? t("update.bulkDialog.nothing")
                  : "";
            return (
              <li key={p.siteId} className={styles.site} data-ready={!why || undefined}>
                <Icon name={why ? (count === 0 && !p.problem ? "ok" : "lock") : "update"} size={14} />
                <span className={styles.name}>{p.name}</span>
                <span className={styles.note}>{why || t("update.bulkDialog.items", { count })}</span>
              </li>
            );
          })}
        </ul>
      ) : (
        <p role="status" className={styles.loading}>
          {t("update.bulkDialog.loading")}
        </p>
      )}
    </Dialog>
  );
}
