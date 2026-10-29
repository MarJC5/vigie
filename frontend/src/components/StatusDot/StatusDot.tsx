import { useTranslation } from "react-i18next";

import styles from "./StatusDot.module.css";

export type Status = "ok" | "warn" | "danger" | "unknown";

// Each status has its own shape, so it never depends on color alone.
const shapes: Record<Status, string> = { ok: "circle", warn: "square", danger: "triangle", unknown: "ring" };

export interface StatusDotProps {
  status: Status;
  /** Shows the status name next to the dot; without it the dot is announced on its own. */
  withLabel?: boolean;
  /** Replaces the status name, for example "Passed" for the result of a check. */
  label?: string;
}

export function StatusDot({ status, withLabel = false, label: own }: StatusDotProps) {
  const { t } = useTranslation();
  const label = own ?? t(`status.${status}`);
  const dot = (
    <span
      className={`${styles.dot} ${styles[status]}`}
      data-shape={shapes[status]}
      {...(withLabel ? { "aria-hidden": true } : { role: "img", "aria-label": label })}
    />
  );
  if (!withLabel) return dot;
  return (
    <span className={styles.labelled}>
      {dot}
      <span>{label}</span>
    </span>
  );
}
