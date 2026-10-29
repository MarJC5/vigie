import type { ReactNode } from "react";

import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./Counts.module.css";

export interface Count {
  label: string;
  value: number;
  /** "warn" marks a count that needs attention, with the warning shape, when it is not zero. */
  tone?: "warn";
}

export interface CountsProps {
  label: string;
  items: Count[];
  /** Actions at the right of the row, for example "Scan host". */
  children?: ReactNode;
}

/** A row of counts with caps labels, for a host or a folder. */
export function Counts({ label, items, children }: CountsProps) {
  return (
    <div className={styles.row} data-counts>
      <ul aria-label={label} className={styles.counts}>
      {items.map((item) => (
        <li key={item.label} className={styles.count}>
          <span className={styles.label}>{item.label}</span>
          <span className={styles.value}>
            {item.tone === "warn" && item.value > 0 && <StatusDot status="warn" />}
            {item.value}
          </span>
        </li>
      ))}
      </ul>
      {children && <div className={styles.actions}>{children}</div>}
    </div>
  );
}
