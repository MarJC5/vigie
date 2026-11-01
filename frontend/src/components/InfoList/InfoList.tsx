import type { ReactNode } from "react";

import styles from "./InfoList.module.css";

/** Label and value rows on faint alternating stripes. */
export function InfoList({ children }: { children: ReactNode }) {
  return <dl className={styles.list}>{children}</dl>;
}

export interface InfoRowProps {
  label: string;
  children: ReactNode;
  /** An inline action after the value, for example an "Update" text button. */
  action?: ReactNode;
  /** A line in gray under the value, saying what it means or why it is missing. */
  note?: ReactNode;
}

export function InfoRow({ label, children, action, note }: InfoRowProps) {
  return (
    <div className={styles.row} data-noted={note ? true : undefined}>
      <dt className={styles.label}>{label}</dt>
      <dd className={styles.value}>
        {note ? (
          <span className={styles.stack}>
            <span className={styles.line}>
              {children}
              {action}
            </span>
            <span className={styles.note}>{note}</span>
          </span>
        ) : (
          <>
            {children}
            {action}
          </>
        )}
      </dd>
    </div>
  );
}

/** A value's complement in gray, on its line: when it was seen, where it is. */
export function InfoAside({ children }: { children: ReactNode }) {
  return <span className={styles.aside}>{children}</span>;
}
