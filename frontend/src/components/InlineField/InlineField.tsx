import type { InputHTMLAttributes, KeyboardEvent } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./InlineField.module.css";

export interface InlineFieldProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "type" | "className" | "style" | "onChange" | "aria-label" | "aria-invalid"> {
  /** Read by screen readers: a field in a row has no visible label. */
  label: string;
  onChange?: (value: string) => void;
  /** Enter, with the typed value. */
  onCommit?: (value: string) => void;
  /** Escape. */
  onCancel?: () => void;
  invalid?: boolean;
  /** A leading icon, inside the field (the tag of a new tag). */
  icon?: IconName;
}

/** A field in a row: rename in the tree, name a site found, add a tag. Plain text at rest, the
 * field's ground on hover and focus, the focus ring when typing (ADR-0004). */
export function InlineField({ label, onChange, onCommit, onCancel, invalid = false, icon, onKeyDown, ...input }: InlineFieldProps) {
  const keys = (event: KeyboardEvent<HTMLInputElement>) => {
    onKeyDown?.(event);
    if (event.defaultPrevented) return;
    if (event.key === "Enter" && onCommit) onCommit(event.currentTarget.value);
    if (event.key === "Escape" && onCancel) onCancel();
  };
  const field = (
    <input
      {...input}
      type="text"
      aria-label={label}
      aria-invalid={invalid || undefined}
      spellCheck={input.spellCheck ?? false}
      className={styles.input}
      data-icon={icon ? true : undefined}
      onChange={onChange && ((event) => onChange(event.target.value))}
      onKeyDown={keys}
    />
  );
  if (!icon) return field;
  return (
    <span className={styles.withIcon}>
      <span className={styles.icon} aria-hidden="true">
        <Icon name={icon} size={12} />
      </span>
      {field}
    </span>
  );
}
