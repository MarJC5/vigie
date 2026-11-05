import { useTranslation } from "react-i18next";

import type { ExportSummary, Node, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { countsText, exportOptions, sizeText } from "../../lib/data";
import { Button } from "../Button/Button";
import { Checkbox } from "../Checkbox/Checkbox";
import { ChoiceCard } from "../ChoiceCard/ChoiceCard";
import { Dialog } from "../Dialog/Dialog";
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
        <div className={styles.cards}>
          <ChoiceCard
            title={t("data.export.all")}
            text={all && counts ? counts : t("data.export.allHint")}
            selected={all}
            onSelect={() => onNode("")}
          />
          <ChoiceCard
            title={t("data.export.node")}
            text={!all && counts ? counts : t("data.export.nodeHint")}
            selected={!all}
            disabled={options.length === 0}
            onSelect={() => all && onNode(options[0]?.value ?? "")}
          />
        </div>
        {!all && <SelectField label={t("data.export.node")} hideLabel value={nodeId} options={options} onChange={onNode} />}
      </fieldset>
      <Checkbox
        label={t("data.export.history")}
        note={
          summary
            ? t("data.export.historyHint", { size: sizeText(summary.historyBytes, i18n.language) })
            : t("data.export.historyHintShort")
        }
        checked={history}
        onChange={onHistory}
      />
      <Notice tone="neutral" icon="lock" role="note" title={t("data.export.sensitive")} text={t("data.export.sensitiveText")} />
    </Dialog>
  );
}
