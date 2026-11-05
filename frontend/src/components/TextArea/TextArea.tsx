import { useId } from "react";

import { FieldNote } from "../FieldNote/FieldNote";
import styles from "./TextArea.module.css";

export interface TextAreaProps {
  label: string;
  /** The label is read by screen readers but not shown (the notes of a site, under their title). */
  hideLabel?: boolean;
  value: string;
  onChange: (value: string) => void;
  /** Help under the field, read with it. */
  hint?: string;
  /** Replaces the hint and marks the field invalid. */
  error?: string;
  /** The error is a refusal after sending: announced as it comes. */
  alert?: boolean;
  /** Marked invalid without a note (a limit said elsewhere). */
  invalid?: boolean;
  required?: boolean;
  /** text: TextField on a few lines; notes: monospace, tall, resized by the user. */
  variant?: "text" | "notes";
  rows?: number;
  autoFocus?: boolean;
  spellCheck?: boolean;
}

/** TextField's look on several lines, with its hint or error under it, both read with the field. */
export function TextArea({
  label,
  hideLabel = false,
  value,
  onChange,
  hint,
  error,
  alert = false,
  invalid = false,
  required,
  variant = "text",
  rows = 2,
  autoFocus,
  spellCheck = false,
}: TextAreaProps) {
  const id = useId();
  const note = error ?? hint;
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={hideLabel ? styles.hidden : styles.label}>
        {label}
      </label>
      <textarea
        id={id}
        rows={rows}
        value={value}
        required={required}
        autoFocus={autoFocus}
        spellCheck={spellCheck}
        aria-invalid={error || invalid ? true : undefined}
        aria-describedby={note ? `${id}-note` : undefined}
        className={styles.input}
        data-variant={variant}
        onChange={(event) => onChange(event.target.value)}
      />
      {note && (
        <FieldNote id={`${id}-note`} error={!!error} announce={error && alert ? "alert" : undefined}>
          {note}
        </FieldNote>
      )}
    </div>
  );
}
