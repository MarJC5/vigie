import { Icon } from "../Icon/Icon";
import styles from "./ChoiceCard.module.css";

export interface ChoiceCardProps {
  title: string;
  text: string;
  selected: boolean;
  onSelect: () => void;
  disabled?: boolean;
}

/** One of several exclusive choices, as a card; the chosen one has a check in a corner. */
export function ChoiceCard({ title, text, selected, onSelect, disabled = false }: ChoiceCardProps) {
  return (
    <button type="button" aria-pressed={selected} disabled={disabled} className={styles.card} onClick={onSelect}>
      {selected && (
        <span className={styles.corner} aria-hidden="true">
          <Icon name="check" size={12} />
        </span>
      )}
      <span className={styles.title}>{title}</span>
      <span className={styles.text}>{text}</span>
    </button>
  );
}
