import { Checkbox } from "../Checkbox/Checkbox";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./InstallList.module.css";

export interface InstallItem {
  path: string;
  name: string;
  /** What discovery says about it, already translated. */
  state: string;
  detail: string;
  checked: boolean;
  /** Only new installs can be added; the others are shown, not offered. */
  selectable: boolean;
  /** Shown as a problem (for example: cannot be loaded). */
  problem?: boolean;
}

export interface InstallListProps {
  label: string;
  columns: { add: string; name: string; path: string; state: string };
  /** Starts the name of each checkbox: "Add" and the install's name. */
  addLabel: string;
  items: InstallItem[];
  onToggle: (path: string) => void;
  onRename: (path: string, name: string) => void;
}

/** The WordPress installs found on an account: the new ones checked and renamable. */
export function InstallList({ label, columns, addLabel, items, onToggle, onRename }: InstallListProps) {
  return (
    <table aria-label={label} className={styles.table}>
      <thead>
        <tr>
          <th scope="col" className={styles.check}>
            <span className={styles.hidden}>{columns.add}</span>
          </th>
          <th scope="col">{columns.name}</th>
          <th scope="col">{columns.path}</th>
          <th scope="col">{columns.state}</th>
        </tr>
      </thead>
      <tbody>
        {items.map((item) => (
          <tr key={item.path} data-selectable={item.selectable} data-problem={item.problem || undefined}>
            <td className={styles.check}>
              {item.selectable && (
                <Checkbox
                  label={`${addLabel} ${item.name}`}
                  hideLabel
                  checked={item.checked}
                  onChange={() => onToggle(item.path)}
                />
              )}
            </td>
            <td>
              {item.selectable ? (
                <input
                  type="text"
                  aria-label={`${columns.name} ${item.path}`}
                  value={item.name}
                  spellCheck={false}
                  className={styles.name}
                  onChange={(event) => onRename(item.path, event.target.value)}
                />
              ) : (
                <span className={styles.fixed}>{item.name}</span>
              )}
            </td>
            <td className={styles.path} data-selectable>
              {item.path}
            </td>
            <td>
              <span className={styles.state}>
                {item.problem ? <StatusDot status="danger" label={item.state} withLabel /> : item.state}
              </span>
              {item.detail && <span className={styles.detail} data-selectable>
                  {item.detail}
                </span>}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  );
}
