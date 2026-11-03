import { useTranslation } from "react-i18next";

import { durationText, rowText, type RunRow, seconds } from "../../lib/updates";
import { StepList } from "../StepList/StepList";

export interface RunStepsProps {
  label: string;
  rows: RunRow[];
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  /** The run is over: what it did not reach was not started. */
  ended: boolean;
}

/** The steps of an update or a rollback as they go: done, under way, failed, or not reached. */
export function RunSteps({ label, rows, names, ended }: RunStepsProps) {
  const { t } = useTranslation();
  return (
    <StepList
      label={label}
      ended={ended}
      stateLabels={{
        done: t("update.state.done"),
        running: t("update.state.running"),
        failed: t("update.state.failed"),
        waiting: t("update.state.waiting"),
      }}
      rows={rows.map((row) => {
        const { label: text, detail } = rowText(t, row, names, ended);
        const step = row.step;
        return {
          key: row.key,
          state: row.state,
          label: text,
          detail,
          end: step && step.finished ? durationText(t, seconds(step.started, step.finished)) : "",
        };
      })}
    />
  );
}
