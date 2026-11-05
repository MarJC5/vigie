import { type InputHTMLAttributes, useId } from "react";

import { FieldNote } from "../FieldNote/FieldNote";
import styles from "./TextField.module.css";

export interface TextFieldProps {
  label: string;
  value: string;
  onChange: (value: string) => void;
  /** Help under the field, read with it. */
  hint?: string;
  /** Replaces the hint and marks the field invalid. */
  error?: string;
  placeholder?: string;
  inputMode?: InputHTMLAttributes<HTMLInputElement>["inputMode"];
  autoFocus?: boolean;
}

/** A labelled text field with an optional hint or error, both announced with the field. */
export function TextField({ label, value, onChange, hint, error, placeholder, inputMode, autoFocus }: TextFieldProps) {
  const id = useId();
  const note = error ?? hint;
  return (
    <div className={styles.field}>
      <label htmlFor={id} className={styles.label}>
        {label}
      </label>
      <input
        id={id}
        type="text"
        value={value}
        placeholder={placeholder}
        inputMode={inputMode}
        autoFocus={autoFocus}
        spellCheck={false}
        autoCapitalize="off"
        autoCorrect="off"
        aria-invalid={error ? true : undefined}
        aria-describedby={note ? `${id}-note` : undefined}
        className={styles.input}
        onChange={(event) => onChange(event.target.value)}
      />
      {/* Only when there is something to say: an empty line would space the fields unevenly. */}
      {note && (
        <FieldNote id={`${id}-note`} error={!!error}>
          {note}
        </FieldNote>
      )}
    </div>
  );
}
