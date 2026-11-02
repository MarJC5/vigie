import { Icon, type IconName } from "../Icon/Icon";
import styles from "./ItemList.module.css";

export interface ListedItem {
  key: string;
  icon: IconName;
  name: string;
  /** A short note at the end of the row. */
  note?: string;
}

/** The items a dialog acts on, one per row, every other row shaded. */
export function ItemList({ items, label }: { items: ListedItem[]; label?: string }) {
  return (
    <ul className={styles.items} aria-label={label}>
      {items.map((item) => (
        <li key={item.key} className={styles.item}>
          <Icon name={item.icon} size={14} />
          <span className={styles.name}>{item.name}</span>
          {item.note && <span className={styles.note}>{item.note}</span>}
        </li>
      ))}
    </ul>
  );
}
