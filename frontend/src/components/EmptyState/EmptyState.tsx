import type { ReactNode } from "react";

import styles from "./EmptyState.module.css";

export interface EmptyStateProps {
  title: string;
  text: string;
  /** The actions: one primary button, optionally a text button. */
  children?: ReactNode;
}

export function EmptyState({ title, text, children }: EmptyStateProps) {
  return (
    <section className={styles.empty}>
      <h1 className={styles.title}>{title}</h1>
      <p className={styles.text}>{text}</p>
      {children && <div className={styles.actions}>{children}</div>}
    </section>
  );
}
