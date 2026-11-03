import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./CheckList.module.css";

export type CheckState = "ok" | "warn" | "failed" | "waiting" | "running";

export interface CheckItem {
  label: string;
  value: string;
  state: CheckState;
}

export interface CheckListProps {
  label: string;
  items: CheckItem[];
  /** What each state is called, for screen readers and next to the dot. */
  stateLabels: Record<CheckState, string>;
}

const dots: Record<CheckState, Status> = { ok: "ok", warn: "warn", failed: "danger", waiting: "unknown", running: "unknown" };

/** The checks of a test, each with its result: the shape of the dot, never color alone. */
export function CheckList({ label, items, stateLabels }: CheckListProps) {
  return (
    <ol aria-label={label} className={styles.list}>
      {items.map((item) => (
        <li key={item.label} className={styles.item} data-state={item.state}>
          <StatusDot status={dots[item.state]} label={stateLabels[item.state]} />
          <span className={styles.label}>{item.label}</span>
          <span className={styles.value}>{item.value || stateLabels[item.state]}</span>
        </li>
      ))}
    </ol>
  );
}
