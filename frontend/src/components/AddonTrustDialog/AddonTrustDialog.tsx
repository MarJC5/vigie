import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import styles from "./AddonTrustDialog.module.css";

export interface AddonTrustDialogProps {
  onClose: () => void;
}

/** What an add-on can and cannot do, opened by the lock button: not a box that takes the page's room. */
export function AddonTrustDialog({ onClose }: AddonTrustDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog
      title={t("addons.trust")}
      onClose={onClose}
      actions={
        <Button variant="primary" onClick={onClose}>
          {t("addons.understood")}
        </Button>
      }
    >
      <span className={styles.icon}>
        <Icon name="lock" size="lg" />
      </span>
      {(["trust1", "trust2", "trust3"] as const).map((key) => (
        <p key={key} className={styles.text}>
          {t(`addons.${key}`)}
        </p>
      ))}
    </Dialog>
  );
}
