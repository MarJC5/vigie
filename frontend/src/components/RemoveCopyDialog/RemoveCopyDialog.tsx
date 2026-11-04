import { useTranslation } from "react-i18next";

import type { CopyView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { ItemList } from "../ItemList/ItemList";

export interface RemoveCopyDialogProps {
  copy: CopyView;
  /** No trash took it: the dialog asks again, to delete it for good. */
  forGood?: boolean;
  onRemove: () => void;
  onClose: () => void;
}

/** Asks before a copy goes to the Trash (mockup 28i), or, when no trash takes it, before it is
 deleted for good (ADR-0041, point 7). */
export function RemoveCopyDialog({ copy, forGood = false, onRemove, onClose }: RemoveCopyDialogProps) {
  const { t, i18n } = useTranslation();
  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "short", timeStyle: "short" });
  const key = forGood ? "copies.forGood" : "copies.removeDialog";
  return (
    <Dialog
      title={t(`${key}.title`)}
      lead={t(`${key}.lead`)}
      onClose={onClose}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("edit.cancel")}
          </Button>
          <Button variant="danger" onClick={onRemove}>
            {t(`${key}.confirm`)}
          </Button>
        </>
      }
    >
      <ItemList
        items={[
          {
            key: copy.folder,
            icon: "archive",
            name: `${when.format(new Date(copy.started))}, ${t(`copies.content.${copy.content}`).toLowerCase()}`,
            note: sizeText(copy.bytes, i18n.language),
          },
        ]}
      />
    </Dialog>
  );
}
