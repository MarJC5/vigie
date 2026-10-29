import { type CSSProperties, useId } from "react";

import styles from "./Progress.module.css";

export interface ProgressProps {
  /** What is happening, shown and announced. */
  label: string;
  /** With max, a counted progress; without, the length of the wait is unknown. */
  value?: number;
  max?: number;
}

/** A wait the user can see: a bar under a sentence saying what is happening. */
export function Progress({ label, value, max }: ProgressProps) {
  const id = useId();
  const counted = value !== undefined && max !== undefined && max > 0;
  return (
    <div className={styles.progress}>
      <span id={id} className={styles.label}>
        {label}
      </span>
      <div
        role="progressbar"
        aria-labelledby={id}
        aria-valuemin={counted ? 0 : undefined}
        aria-valuemax={counted ? max : undefined}
        aria-valuenow={counted ? value : undefined}
        className={styles.track}
      >
        {/* A new element when the form changes, so the moving segment never shrinks into a count. */}
        <span
          key={counted ? "counted" : "moving"}
          className={counted ? styles.fill : styles.moving}
          style={counted ? ({ "--share": `${(100 * value) / max}%` } as CSSProperties) : undefined}
        />
      </div>
    </div>
  );
}
