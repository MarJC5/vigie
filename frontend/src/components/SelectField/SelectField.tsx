import { useId } from "react";

import { Icon } from "../Icon/Icon";
import styles from "./SelectField.module.css";

export interface SelectFieldProps {
  label: string;
  value: string;
  options: Array<{ value: string; label: string }>;
  onChange: (value: string) => void;
}

/** A labelled select: the system's own list, keyboard and screen reader behavior, drawn like the other fields. */
export function SelectField({ label, value, options, onChange }: SelectFieldProps) {
  const id = useId();
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
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
