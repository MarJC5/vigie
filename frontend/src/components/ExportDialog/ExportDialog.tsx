import { useId } from "react";
import { useTranslation } from "react-i18next";

import type { ExportSummary, Node, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { countsText, exportOptions, sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Dialog } from "../Dialog/Dialog";
import { Icon } from "../Icon/Icon";
import { Notice } from "../Notice/Notice";
import { SelectField } from "../SelectField/SelectField";
import styles from "./ExportDialog.module.css";

export interface ExportDialogProps {
  /** The site manager, depth first: its folders and hosts can be exported alone. */
  nodes: Node[];
  /** What is exported: "" for everything, else a folder or host. */
  nodeId: string;
  onNode: (nodeId: string) => void;
  history: boolean;
  onHistory: (history: boolean) => void;
  /** What the export holds; null while Vigie counts it. */
  summary: ExportSummary | null;
  /** Asks where to save, then writes the file. */
  onExport: () => void;
  onClose: () => void;
  /** The file dialog is open, or the file being written. */
  busy?: boolean;
  problem?: Problem | null;
}

/** Export the site manager, or a folder or host of it, with or without the history. */
export function ExportDialog({ nodes, nodeId, onNode, history, onHistory, summary, onExport, onClose, busy = false, problem }: ExportDialogProps) {
  const { t, i18n } = useTranslation();
  const group = useId();
  const options = exportOptions(nodes);
  const counts =
    summary &&
    countsText(t, [
      ["folders", summary.folders],
      ["hosts", summary.hosts],
      ["sites", summary.sites],
      ["ignores", summary.ignores],
    ]);
  const all = nodeId === "";
  return (
    <Dialog
      title={t("data.export.title")}
      lead={t("data.export.lead")}
      onClose={onClose}
      wide
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button variant="primary" disabled={busy || !summary || options.length === 0} aria-busy={busy} onClick={onExport}>
            {t("data.export.confirm")}
          </Button>
        </>
      }
    >
      {problem && <Notice title={t("data.export.failed")} text={t(problem.code, { message: problem.detail })} />}
      <fieldset className={styles.group}>
        <legend className={styles.legend}>{t("data.export.what")}</legend>
        <label className={styles.choice} data-selected={all || undefined}>
          <input type="radio" name={group} className={styles.radio} checked={all} onChange={() => onNode("")} />
          <span className={styles.text}>
            <span className={styles.title}>{t("data.export.all")}</span>
            {all && counts && <span className={styles.hint}>{counts}</span>}
          </span>
        </label>
        <label className={styles.choice} data-selected={!all || undefined}>
          <input
            type="radio"
            name={group}
            className={styles.radio}
            checked={!all}
            disabled={options.length === 0}
            onChange={() => onNode(options[0]?.value ?? "")}
          />
          <span className={styles.text}>
            <span className={styles.title}>{t("data.export.node")}</span>
            {!all && counts && <span className={styles.hint}>{counts}</span>}
          </span>
          {!all && (
            <span className={styles.select}>
              <SelectField label={t("data.export.node")} hideLabel value={nodeId} options={options} onChange={onNode} />
            </span>
          )}
        </label>
      </fieldset>
      <label className={styles.check}>
        <input type="checkbox" className={styles.box} checked={history} onChange={(event) => onHistory(event.target.checked)} />
        <span className={styles.text}>
          <span className={styles.title}>{t("data.export.history")}</span>
          <span className={styles.hint}>
            {summary
              ? t("data.export.historyHint", { size: sizeText(summary.historyBytes, i18n.language) })
              : t("data.export.historyHintShort")}
          </span>
        </span>
      </label>
      <div className={styles.sensitive}>
        <Icon name="lock" size={18} />
        <span className={styles.text}>
          <span className={styles.sensitiveTitle}>{t("data.export.sensitive")}</span>
          <span>{t("data.export.sensitiveText")}</span>
        </span>
      </div>
    </Dialog>
  );
}
