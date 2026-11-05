import type { KeyboardEvent, Ref } from "react";

import { Icon } from "../Icon/Icon";
import styles from "./SearchField.module.css";

export interface SearchFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** Escape on an empty field. */
  onClose: () => void;
  ref?: Ref<HTMLInputElement>;
  autoFocus?: boolean;
}

/** A search field: Escape clears it, then closes it. */
export function SearchField({ label, value, onChange, onClose, ref, autoFocus }: SearchFieldProps) {
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Escape") return;
    event.preventDefault();
    if (value) onChange("");
    else onClose();
  };
  return (
    <span className={styles.field}>
      <Icon name="search" size="sm" />
      <input
        ref={ref}
        type="search"
        aria-label={label}
        placeholder={label}
        value={value}
        spellCheck={false}
        autoFocus={autoFocus}
        className={styles.input}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={onKeyDown}
      />
    </span>
  );
}
