import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { LinkButton } from "../LinkButton/LinkButton";
import { TextField } from "../TextField/TextField";
import styles from "./AdminAddressDialog.module.css";

export interface AdminAddressDialogProps {
  /** The address the user gave, "" when the computed one is used. */
  own: string;
  /** The WordPress address followed by /wp-admin/, "" when none is known yet. */
  computed: string;
  /** Saves an address, "" for the computed one; a problem keeps the dialog open. */
  onSave: (address: string) => Promise<Problem | null>;
  onClose: () => void;
}

/** The admin address of a site whose login a plugin renamed (ADR-0023, amended). */
export function AdminAddressDialog({ own, computed, onSave, onClose }: AdminAddressDialogProps) {
  const { t } = useTranslation();
  const [address, setAddress] = useState(own || computed);
  const [problem, setProblem] = useState<Problem | null>(null);
  const save = async (value: string) => {
    const refused = await onSave(value);
    if (refused) setProblem(refused);
  };
  return (
    <Dialog
      title={t("admin.title")}
      lead={t("admin.lead")}
      wide
      onClose={onClose}
      actions={
        <>
          {own && (
            <LinkButton className={styles.reset} onClick={() => void save("")}>
              {t("admin.reset")}
            </LinkButton>
          )}
          <Button onClick={onClose}>{t("action.cancel")}</Button>
          <Button variant="primary" onClick={() => void save(address.trim() === computed ? "" : address)}>
            {t("admin.save")}
          </Button>
        </>
      }
    >
      <TextField
        label={t("admin.label")}
        value={address}
        onChange={(value) => {
          setAddress(value);
          setProblem(null);
        }}
        inputMode="url"
        hint={computed ? t("admin.computed", { address: computed }) : undefined}
        error={problem ? t(problem.code) : undefined}
      />
    </Dialog>
  );
}
