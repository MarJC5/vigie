import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./FilterPad.module.css";

export interface FilterPadProps {
  label: string;
  count: number;
  /** inline: a label and a muted count (the filters of the Plugins tab); column: a small label
   * above a 22 px value (the counts of a host or a folder). */
  layout?: "inline" | "column";
  /** In a column, the status's mark beside a value that is not zero. */
  tone?: "warn" | "danger";
  pressed?: boolean;
  /** Without it, the pad shows a count and filters nothing (a total), with the pad's room. */
  onSelect?: () => void;
}

/** The pad of a filter: hover on the row's ground, the chosen one in bold on the selection's ground,
 * the focus ring (ADR-0004). */
export function FilterPad({ label, count, layout = "inline", tone, pressed = false, onSelect }: FilterPadProps) {
  const body =
    layout === "column" ? (
      <>
        <span className={styles.label}>{label}</span>
        <span className={styles.value} data-value>
          {tone && count > 0 && <StatusDot status={tone} />}
          {count}
        </span>
      </>
    ) : (
      <>
        {label}
        <span className={styles.count}>{count}</span>
      </>
    );
  if (!onSelect) {
    return (
      <span className={styles.pad} data-layout={layout}>
        {body}
      </span>
    );
  }
  return (
    <button type="button" className={styles.pad} data-layout={layout} aria-pressed={pressed} onClick={onSelect}>
      {body}
    </button>
  );
}
