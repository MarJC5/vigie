import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { FindingView, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "../../lib/findings";
import { Button } from "../Button/Button";
import { ChoiceCard } from "../ChoiceCard/ChoiceCard";
import { Dialog } from "../Dialog/Dialog";
import styles from "./IgnoreDialog.module.css";

export interface IgnoreDialogProps {
  /** A finding to set aside, or an update offered to ignore (ADR-0026). */
  finding?: FindingView;
  update?: { name: string; from: string; to: string };
  site: string;
  /** untilVersion "" ignores permanently, and then a reason is required. */
  onIgnore: (untilVersion: string, reason: string) => void;
  onClose: () => void;
  /** Why the last attempt was refused. */
  problem?: Problem | null;
}

// "Until the next version" only for what has versions of its own: WordPress, a plugin, a theme, PHP.
const versioned = (f: FindingView) => f.version !== "" && /^(core|php|plugin:|theme:)/.test(f.subject);

/** Sets a finding aside, or ignores an update offered, until the next version or permanently with
 a reason. */
export function IgnoreDialog({ finding, update, site, onIgnore, onClose, problem }: IgnoreDialogProps) {
  const { t, i18n } = useTranslation();
  const id = useId();
  // An update offered waits for a newer one; a finding, for another version of its subject.
  const version = update ? update.to : finding!.version;
  const canWait = update ? true : versioned(finding!);
  const [forever, setForever] = useState(!canWait);
  const [reason, setReason] = useState("");
  const name = update
    ? update.name
    : finding!.name || (finding!.subject === "core" ? "WordPress" : finding!.subject === "php" ? "PHP" : finding!.subject.replace(/^\w+:/, ""));
  const missing = forever && reason.trim() === "";

  return (
    <Dialog
      title={update ? t("update.ignoreDialog.title", { name }) : t("security.dialog.title", { title: findingText(finding!, t, i18n.language).title })}
      lead={update ? t("update.ignoreDialog.lead", { from: update.from, to: update.to }) : t("security.dialog.lead", { site })}
      onClose={onClose}
      wide
      actions={
        <>
          <Button onClick={onClose}>{t("action.cancel")}</Button>
          <Button variant="primary" icon="eyeSlash" disabled={missing} onClick={() => onIgnore(forever ? "" : version, reason.trim())}>
            {t("security.dialog.confirm")}
          </Button>
        </>
      }
    >
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t("security.dialog.how")}</legend>
        <div className={styles.cards}>
          {canWait && (
            <ChoiceCard
              title={update ? t("update.ignoreDialog.untilNext") : t("security.dialog.untilNext", { name })}
              text={t(update ? "update.ignoreDialog.untilNextHint" : "security.dialog.untilNextHint", { version })}
              selected={!forever}
              onSelect={() => setForever(false)}
            />
          )}
          <ChoiceCard
            title={t("security.dialog.forever")}
            text={t("security.dialog.foreverHint")}
            selected={forever}
            onSelect={() => setForever(true)}
          />
        </div>
      </fieldset>
      <div className={styles.field}>
        <label htmlFor={`${id}-reason`} className={styles.label}>
          {forever ? t("security.dialog.reason") : t("security.dialog.reasonOptional")}
        </label>
        <textarea
          id={`${id}-reason`}
          rows={2}
          value={reason}
          required={forever}
          aria-describedby={problem ? `${id}-problem` : undefined}
          className={styles.input}
          onChange={(event) => setReason(event.target.value)}
        />
        {problem && (
          <span id={`${id}-problem`} role="alert" className={styles.problem}>
            {t(problem.code, { message: problem.detail })}
          </span>
        )}
      </div>
    </Dialog>
  );
}
