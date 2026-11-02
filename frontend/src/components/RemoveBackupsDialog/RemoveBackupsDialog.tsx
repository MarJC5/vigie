import { useTranslation } from "react-i18next";

import type { BackupView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { ItemList } from "../ItemList/ItemList";

export interface RemoveBackupsDialogProps {
  /** The site's name. */
  name: string;
  /** The backups the limits remove. */
  backups: BackupView[];
  onRemove: () => void;
  onClose: () => void;
}

/** Asks before removing on the server a site's backups past the limits (ADR-0033). */
export function RemoveBackupsDialog({ name, backups, onRemove, onClose }: RemoveBackupsDialogProps) {
  const { t, i18n } = useTranslation();
  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "short", timeStyle: "short" });
  return (
    <Dialog
      title={t("backups.remove.title", { count: backups.length, name })}
      lead={t("backups.remove.lead")}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          <Button variant="danger" onClick={onRemove}>
            {t("backups.remove.confirm", { count: backups.length })}
          </Button>
        </>
      }
    >
      <ItemList
        items={backups.map((b) => ({ key: b.at, icon: "archive", name: when.format(new Date(b.at)), note: sizeText(b.bytes, i18n.language) }))}
      />
    </Dialog>
  );
}
