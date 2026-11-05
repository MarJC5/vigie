import { useTranslation } from "react-i18next";

import type { SitePreviewView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { ignoredOf, pendingOf, setAsideOf } from "../../lib/updates";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import styles from "./BulkDialog.module.css";

export interface BulkDialogProps {
  /** The host or folder. */
  name: string;
  /** Several items selected (ADR-0034): how many, said in place of a name. */
  count?: number;
  /** The previews of its sites; null while Vigie reads them. */
  previews: SitePreviewView[] | null;
  onConfirm: () => void;
  onClose: () => void;
}

/** Asks before updating every site of a host or folder: what each would change, or why it cannot. */
export function BulkDialog({ name, count, previews, onConfirm, onClose }: BulkDialogProps) {
  const { t } = useTranslation();
  const ready = (previews ?? []).filter((p) => !p.problem && p.dumpTool && pendingOf(p) > 0).length;

  return (
    <Dialog
      title={count !== undefined ? t("update.bulkDialog.titleMany", { count }) : t("update.bulkDialog.title", { name })}
      lead={t("update.bulkDialog.lead")}
      onClose={onClose}
      wide
      progress={previews ? undefined : { label: t("update.bulkDialog.loading") }}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button variant="primary" icon="update" onClick={onConfirm} disabled={ready === 0} aria-busy={!previews || undefined}>
            {previews ? t("update.bulkDialog.confirm", { count: ready }) : t("update.updateAll")}
          </Button>
        </>
      }
    >
      {previews && (
        <ul aria-label={t("update.bulkDialog.sites")} className={styles.sites}>
          {previews.map((p) => {
            const count = pendingOf(p);
            // The updates the site ignores (ADR-0026) and the versions set aside (ADR-0042) are left
            // out, and said.
            const ignored = ignoredOf(p).length;
            const aside = setAsideOf(p).length;
            const why = p.problem
              ? t(p.problem.code, { message: p.problem.detail })
              : !p.dumpTool
                ? t("problem.noDumpTool")
                : count === 0
                  ? ignored > 0
                    ? t("update.bulkDialog.onlyIgnored", { count: ignored })
                    : aside > 0
                      ? t("update.bulkDialog.onlySetAside", { count: aside })
                      : t("update.bulkDialog.nothing")
                  : "";
            const items = [
              t("update.bulkDialog.items", { count }),
              ...(ignored > 0 ? [t("update.bulkDialog.ignoredLeft", { count: ignored })] : []),
              ...(aside > 0 ? [t("update.bulkDialog.setAsideLeft", { count: aside })] : []),
            ].join(" · ");
            const idle = ignored > 0 ? "eyeSlash" : aside > 0 ? "rollback" : "ok";
            return (
              <li key={p.siteId} className={styles.site} data-ready={!why || undefined}>
                <Icon name={why ? (count === 0 && !p.problem ? idle : "lock") : "update"} size={14} />
                <span className={styles.name}>{p.name}</span>
                <span className={styles.note}>{why || items}</span>
              </li>
            );
          })}
        </ul>
      )}
    </Dialog>
  );
}
