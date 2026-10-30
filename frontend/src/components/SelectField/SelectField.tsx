import { useId } from "react";

import { Icon } from "../Icon/Icon";
import styles from "./SelectField.module.css";

export interface SelectFieldProps {
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
  /** The label is read but not shown, for a row that already names the choice. */
  hideLabel?: boolean;
}

/** A labelled select: the system's own list, keyboard and screen reader behavior, drawn like the other fields. */
export function SelectField({ label, value, options, onChange, hideLabel = false }: SelectFieldProps) {
  const id = useId();
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={hideLabel ? styles.hidden : styles.label}>
        {label}
      </label>
      <span className={styles.control}>
        <select id={id} value={value} className={styles.select} onChange={(event) => onChange(event.target.value)}>
          {options.map((o) => (
            <option key={o.value} value={o.value}>
              {o.label}
            </option>
          ))}
        </select>
        <span className={styles.caret} aria-hidden="true">
          <Icon name="caretDown" size={14} />
        </span>
      </span>
    </div>
  );
}
