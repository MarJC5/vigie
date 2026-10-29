import type { CSSProperties, ReactNode } from "react";

import styles from "./Columns.module.css";

export interface ColumnsProps {
  /** Equal tracks; children span them with Column. */
  count: number;
  children: ReactNode;
}

/** A grid of equal columns, for forms and side by side choices. */
export function Columns({ count, children }: ColumnsProps) {
  return (
    <div className={styles.columns} style={{ "--columns": count } as CSSProperties}>
      {children}
    </div>
  );
}

export interface ColumnProps {
  span?: number;
  children: ReactNode;
}

export function Column({ span = 1, children }: ColumnProps) {
  return (
    <div className={styles.column} style={{ "--span": span } as CSSProperties}>
      {children}
    </div>
  );
}
