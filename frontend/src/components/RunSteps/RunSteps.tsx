import { useTranslation } from "react-i18next";

import { durationText, rowText, type RowState, type RunRow, seconds } from "../../lib/updates";
import { Icon, type IconName } from "../Icon/Icon";
import styles from "./RunSteps.module.css";

export interface RunStepsProps {
  label: string;
  rows: RunRow[];
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  /** The run is over: what it did not reach was not started. */
  ended: boolean;
}

const icons: Record<RowState, IconName> = { done: "ok", running: "spinner", failed: "failed", waiting: "circle" };

/** The steps of an update or a rollback as they go: done, under way, failed, or not reached. */
export function RunSteps({ label, rows, names, ended }: RunStepsProps) {
  const { t } = useTranslation();
  return (
    <ol aria-label={label} className={styles.steps}>
      {rows.map((row) => {
        const { label: text, detail } = rowText(t, row, names, ended);
        const step = row.step;
        const took = step && step.finished ? durationText(t, seconds(step.started, step.finished)) : "";
        return (
          <li key={row.key} className={styles.step} data-state={row.state} aria-current={row.state === "running" ? "step" : undefined}>
            <span className={styles.icon}>
              <Icon name={icons[row.state]} size={18} label={t(`update.state.${row.state}`)} />
            </span>
            <span className={styles.label}>{text}</span>
            <span className={styles.detail}>{detail}</span>
            <span className={styles.time}>{took}</span>
          </li>
        );
      })}
    </ol>
  );
}
