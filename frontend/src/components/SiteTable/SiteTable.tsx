import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./SiteTable.module.css";

export interface SiteRowData {
  id: string;
  name: string;
  /** ok, warn or unknown from the latest scan; empty when never scanned. */
  status: string;
  wordpress: string;
  php: string;
  pending: number;
}

export interface SiteTableProps {
  label: string;
  columns: { site: string; status: string; wordpress: string; php: string; updates: string };
  notScanned: string;
  rows: SiteRowData[];
  onSelect: (id: string) => void;
  /** Opens a site's menu at the point of a right-click on its row. */
  onMenu?: (id: string, point: { x: number; y: number }) => void;
}

/** The sites of a host or folder with their latest scan; a name opens the site. */
export function SiteTable({ label, columns, notScanned, rows, onSelect, onMenu }: SiteTableProps) {
  return (
    // One line per cell; a window too narrow scrolls the table, not the page.
    <div className={styles.scroll}>
      <table aria-label={label} className={styles.table}>
        <thead>
          <tr>
            <th scope="col">{columns.site}</th>
            <th scope="col">{columns.status}</th>
            <th scope="col">{columns.wordpress}</th>
            <th scope="col">{columns.php}</th>
            <th scope="col">{columns.updates}</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
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
              <td>{row.status ? <StatusDot status={row.status as Status} withLabel /> : <span className={styles.muted}>{notScanned}</span>}</td>
              <td>{row.wordpress}</td>
              <td>{row.php}</td>
              <td>{row.status ? row.pending : ""}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
