import { useState } from "react";

import { Icon } from "../Icon/Icon";
import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./SiteTable.module.css";

export interface SiteRowData {
  id: string;
  name: string;
  /** ok, warn, danger, partial or unknown from the latest scan; empty when never scanned. */
  status: string;
  /** The worst finding not set aside, in words; empty when none. */
  worst: string;
  pending: number;
  /** Its folders and host, when the table has the column. */
  where?: string;
}

export interface SiteTableProps {
  label: string;
  /** where: its folders and host, for sites from several hosts (a tag's). */
  columns: { site: string; where?: string; status: string; worst: string; updates: string };
  notScanned: string;
  rows: SiteRowData[];
  onSelect: (id: string) => void;
  /** Opens a site's menu at the point of a right-click on its row. */
  onMenu?: (id: string, point: { x: number; y: number }) => void;
}

type SortKey = "site" | "status" | "updates";

// The worst first: a status by how much it needs you; a site never scanned last.
const severity: Record<string, number> = { danger: 5, warn: 4, partial: 3, unknown: 2, ok: 1, "": 0 };

// What a first click shows first: names from A, the worst status, the most updates.
const firstDirection: Record<SortKey, 1 | -1> = { site: 1, status: -1, updates: -1 };

function compare(key: SortKey, a: SiteRowData, b: SiteRowData): number {
  if (key === "status") return (severity[a.status] ?? 0) - (severity[b.status] ?? 0);
  if (key === "updates") return (a.status ? a.pending : -1) - (b.status ? b.pending : -1);
  return a.name.localeCompare(b.name);
}

/** The sites of a host or folder with their latest scan; a name opens the site. */
export function SiteTable({ label, columns, notScanned, rows, onSelect, onMenu }: SiteTableProps) {
  // The order of the site list until a column is clicked; ties keep it (ADR-0023, point 5).
  const [sort, setSort] = useState<{ key: SortKey; direction: 1 | -1 } | null>(null);
  const sorted = sort ? [...rows].sort((a, b) => sort.direction * compare(sort.key, a, b)) : rows;
  const header = (key: SortKey, text: string) => {
    const current = sort?.key === key ? sort.direction : 0;
    // aria-sort says how the values run: the worst status first is descending.
    const aria = current === 0 ? undefined : current === 1 ? "ascending" : "descending";
    return (
      <th scope="col" aria-sort={aria}>
        <button
          type="button"
          className={styles.sort}
          onClick={() => setSort({ key, direction: current === 0 ? firstDirection[key] : current === 1 ? -1 : 1 })}
        >
          {text}
          {current !== 0 && <Icon name={current === firstDirection[key] ? "caretDown" : "caretUp"} size={11} />}
        </button>
      </th>
    );
  };
  return (
    // One line per cell; the worst finding gives way when the window is narrow.
    <div>
      <table aria-label={label} className={styles.table}>
        <thead>
          <tr>
            {header("site", columns.site)}
            {columns.where && <th scope="col">{columns.where}</th>}
            {header("status", columns.status)}
            <th scope="col">{columns.worst}</th>
            {header("updates", columns.updates)}
          </tr>
        </thead>
        <tbody>
          {sorted.map((row) => (
            <tr
              key={row.id}
              onContextMenu={
                onMenu &&
                ((event) => {
                  event.preventDefault();
                  onMenu(row.id, { x: event.clientX, y: event.clientY });
                })
              }
            >
              <td>
                <button type="button" className={styles.name} onClick={() => onSelect(row.id)}>
                  {row.name}
                </button>
              </td>
              {columns.where && <td className={styles.muted}>{row.where}</td>}
              <td>{row.status ? <StatusDot status={row.status as Status} withLabel /> : <span className={styles.muted}>{notScanned}</span>}</td>
              <td className={`${styles.muted} ${styles.worst}`} title={row.worst || undefined}>
                {row.worst}
              </td>
              <td>{row.status ? row.pending : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
