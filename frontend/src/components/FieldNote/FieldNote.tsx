import type { ReactNode } from "react";

import styles from "./FieldNote.module.css";

export interface FieldNoteProps {
  /** For the field's aria-describedby. */
  id?: string;
  children: ReactNode;
  /** An error: in the text's color, the field around it carries the red edge. */
  error?: boolean;
  /** "alert" for a refusal after sending, "polite" for a hint that changes as one types. */
  announce?: "alert" | "polite";
}

/** The note under a field: its hint or its error, the one way the window writes either
 * (TextField, TextArea, a field in a row). */
export function FieldNote({ id, children, error = false, announce }: FieldNoteProps) {
  return (
    <span
      id={id}
      className={error ? styles.error : styles.hint}
      role={announce === "alert" ? "alert" : undefined}
      aria-live={announce === "polite" ? "polite" : undefined}
    >
      {children}
    </span>
  );
}
