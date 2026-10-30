import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { FindingView, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "../../lib/findings";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import styles from "./IgnoreDialog.module.css";

export interface IgnoreDialogProps {
  finding: FindingView;
  site: string;
  /** untilVersion "" ignores permanently, and then a reason is required. */
  onIgnore: (untilVersion: string, reason: string) => void;
  onClose: () => void;
  /** Why the last attempt was refused. */
  problem?: Problem | null;
}

// "Until the next version" only for what has versions of its own: WordPress, a plugin, a theme, PHP.
const versioned = (f: FindingView) => f.version !== "" && /^(core|php|plugin:|theme:)/.test(f.subject);

/** Sets a finding aside, until the next version of its subject or permanently with a reason. */
export function IgnoreDialog({ finding, site, onIgnore, onClose, problem }: IgnoreDialogProps) {
  const { t, i18n } = useTranslation();
  const id = useId();
  const canWait = versioned(finding);
  const [forever, setForever] = useState(!canWait);
  const [reason, setReason] = useState("");
  const title = findingText(finding, t, i18n.language).title;
  const name = finding.name || (finding.subject === "core" ? "WordPress" : finding.subject === "php" ? "PHP" : finding.subject.replace(/^\w+:/, ""));
  const missing = forever && reason.trim() === "";

  return (
    <Dialog
      title={t("security.dialog.title", { title })}
      lead={t("security.dialog.lead", { site })}
      onClose={onClose}
      wide
      actions={
        <>
          <Button onClick={onClose}>{t("action.cancel")}</Button>
          <Button variant="primary" icon="eyeSlash" disabled={missing} onClick={() => onIgnore(forever ? "" : finding.version, reason.trim())}>
            {t("security.dialog.confirm")}
          </Button>
        </>
      }
    >
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t("security.dialog.how")}</legend>
        {canWait && (
          <label className={styles.choice}>
            <input type="radio" name={`${id}-how`} checked={!forever} onChange={() => setForever(false)} className={styles.radio} />
            <span className={styles.choiceText}>
              <span className={styles.choiceTitle}>{t("security.dialog.untilNext", { name })}</span>
              <span className={styles.hint}>{t("security.dialog.untilNextHint", { version: finding.version })}</span>
            </span>
          </label>
        )}
        <label className={styles.choice}>
          <input type="radio" name={`${id}-how`} checked={forever} onChange={() => setForever(true)} className={styles.radio} />
          <span className={styles.choiceText}>
            <span className={styles.choiceTitle}>{t("security.dialog.forever")}</span>
            <span className={styles.hint}>{t("security.dialog.foreverHint")}</span>
          </span>
        </label>
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
