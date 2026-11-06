import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { OperationView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { backupPath, durationText, operationBy, operationSummary, restoredStep, rollbacksOf, rowText, type RowState, seconds } from "../../lib/updates";
import { Button } from "../Button/Button";
import { Icon, type IconName } from "../Icon/Icon";
import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./UpdateHistory.module.css";

export interface UpdateHistoryProps {
  /** The newest first, as Vigie lists them. */
  operations: OperationView[];
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
  onRollback: (operationID: string) => void;
  /** An update or a rollback runs on the site: no rollback can start. */
  busy?: boolean;
}

const statuses: Record<string, Status> = {
  completed: "ok",
  failed: "danger",
  cancelled: "warn",
  interrupted: "warn",
  running: "unknown",
};

const icons: Record<RowState, IconName> = { done: "ok", running: "spinner", failed: "failed", waiting: "circle" };

/** The updates and rollbacks of a site, each with its steps; the newest open, and the rollback of its
 failed item offered. */
export function UpdateHistory({ operations, names, onRollback, busy = false }: UpdateHistoryProps) {
  const { t, i18n } = useTranslation();
  const id = useId();
  // The newest open; under a rollback Vigie ran on its own, the update it came from.
  const [open, setOpen] = useState<Set<string>>(() => {
    const first = operations.find((op) => !op.automatic) ?? operations[0];
    return new Set(first ? [first.id] : []);
  });
  if (operations.length === 0) return <p className={styles.empty}>{t("update.history.empty")}</p>;

  const when = new Intl.DateTimeFormat(i18n.language, { dateStyle: "short", timeStyle: "short" });
  const clock = new Intl.DateTimeFormat(i18n.language, { timeStyle: "medium" });
  const toggle = (opID: string) => {
    const next = new Set(open);
    if (next.has(opID)) next.delete(opID);
    else next.add(opID);
    setOpen(next);
  };

  return (
    <ul aria-label={t("update.history.label")} className={styles.operations}>
      {operations.map((op) => {
        const expanded = open.has(op.id);
        const panel = `${id}-${op.id}`;
        // The items that went back on their own during this update (ADR-0042): told by its outcome,
        // and in its steps after the check that failed; the rollback itself is listed above it.
        const backs = rollbacksOf(op, operations).filter((b) => b.outcome === "completed");
        const outcome =
          op.outcome === "completed" && backs.length > 0
            ? { status: "warn" as Status, label: t("update.history.outcome.rolledBack", { count: backs.length }) }
            : { status: statuses[op.outcome] ?? "unknown", label: t(`update.history.outcome.${op.outcome}`) };
        return (
          <li key={op.id} className={styles.operation} data-open={expanded || undefined}>
            <button type="button" className={styles.head} aria-expanded={expanded} aria-controls={panel} onClick={() => toggle(op.id)}>
              <span className={styles.caret}>
                <Icon name={expanded ? "caretDown" : "caretRight"} size="xs" />
              </span>
              <span className={styles.when}>{when.format(new Date(op.started))}</span>
              <span className={styles.summary}>
                {operationSummary(t, op, names)}
                {operationBy(t, op) && <span className={styles.by}> · {operationBy(t, op)}</span>}
              </span>
              <span className={styles.outcome}>
                <StatusDot status={outcome.status} withLabel label={outcome.label} />
              </span>
              <span className={styles.duration}>{op.finished ? durationText(t, seconds(op.started, op.finished)) : ""}</span>
            </button>
            {expanded && (
              <div id={panel} className={styles.panel}>
                <ol className={styles.steps}>
                  {(op.steps ?? []).flatMap((st) => {
                    const state: RowState = st.outcome === "done" ? "done" : st.outcome === "failed" ? "failed" : st.outcome === "running" ? "running" : "waiting";
                    const { label, detail } = rowText(t, { key: "", state, step: st }, names, true);
                    const rows = [
                      <li key={st.position} className={styles.step} data-state={state}>
                        <span className={styles.time}>{clock.format(new Date(st.started))}</span>
                        <span className={styles.icon}>
                          <Icon name={icons[state]} label={t(`update.state.${state}`)} />
                        </span>
                        <span className={styles.label}>{label}</span>
                        <span className={styles.detail} data-selectable>
                          {detail}
                        </span>
                      </li>,
                    ];
                    const back = st.step === "check" && st.outcome === "failed" ? backs.find((b) => restoredStep(b)?.slug === st.slug) : undefined;
                    const restored = back && restoredStep(back);
                    if (back && restored) {
                      rows.push(
                        <li key={`back-${back.id}`} className={styles.step} data-state="done">
                          <span className={styles.time}>{clock.format(new Date(back.started))}</span>
                          <span className={styles.icon}>
                            <Icon name="rollback" label={t("update.state.done")} />
                          </span>
                          <span className={styles.label}>{rowText(t, { key: "", state: "done", step: restored }, names, true).label}</span>
                          <span className={styles.detail}>{t("update.history.ofItsOwn")}</span>
                        </li>,
                      );
                    }
                    return rows;
                  })}
                </ol>
                <div className={styles.foot}>
                  <p className={styles.backup}>
                    {t("update.history.backupKept")} <span className={styles.path} data-selectable>
                      {backupPath(op.backupDir)}
                    </span>
                  </p>
                  {op.rollback && (
                    <Button variant="primary" icon="rollback" onClick={() => onRollback(op.id)} disabled={busy}>
                      {t("update.result.rollback")}
                    </Button>
                  )}
                </div>
              </div>
            )}
          </li>
        );
      })}
    </ul>
  );
}
