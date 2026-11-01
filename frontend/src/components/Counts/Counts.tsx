import type { ReactNode } from "react";

import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./Counts.module.css";

export interface Count {
  label: string;
  value: number;
  /** Marks a count that needs attention ("warn") or is urgent ("danger") with that status's shape,
   when it is not zero. */
  tone?: "warn" | "danger";
  /** What the count filters on, when it can filter (a status); none for a total. */
  filter?: string;
}

export interface CountsProps {
  label: string;
  items: Count[];
  /** Actions at the right of the row, for example "Scan host". */
  children?: ReactNode;
  /** The filter chosen; undefined for everything. */
  selected?: string;
  /** Makes the counts with a filter buttons: a click chooses it, a second shows everything. */
  onSelect?: (filter: string | undefined) => void;
}

/** A row of counts with caps labels, for a host or a folder. */
export function Counts({ label, items, children, selected, onSelect }: CountsProps) {
  return (
    <div className={styles.row} data-counts>
      <ul aria-label={label} className={styles.counts} data-filters={onSelect ? true : undefined}>
        {items.map((item) => {
          const body = (
            <>
              <span className={styles.label}>{item.label}</span>
              <span className={styles.value}>
                {item.tone && item.value > 0 && <StatusDot status={item.tone} />}
                {item.value}
              </span>
            </>
          );
          const pressed = selected !== undefined && item.filter === selected;
          // A count filters when it has something to show, or shows what it filters.
          const filters = onSelect && item.filter !== undefined && (item.value > 0 || pressed);
          return (
            <li key={item.label} className={styles.item}>
              {filters ? (
                <button type="button" className={styles.count} aria-pressed={pressed} onClick={() => onSelect(pressed ? undefined : item.filter)}>
                  {body}
                </button>
              ) : (
                <span className={styles.count}>{body}</span>
              )}
            </li>
          );
        })}
      </ul>
      {children && <div className={styles.actions}>{children}</div>}
    </div>
  );
}
