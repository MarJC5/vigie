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
}

export function InfoRow({ label, children, action }: InfoRowProps) {
  return (
    <div className={styles.row}>
      <dt className={styles.label}>{label}</dt>
      <dd className={styles.value}>
        {children}
        {action}
      </dd>
    </div>
  );
}
