import styles from "./Checkbox.module.css";

export interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** A sentence under the label, saying what the choice means. */
  note?: string;
  /** The label is read by screen readers but not shown: for the rows of a table. */
  hideLabel?: boolean;
  disabled?: boolean;
}

export interface CheckboxControlProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  disabled?: boolean;
}

/** The drawn box alone, for a row whose own <label> holds it with several cells (the keys of an
 * import, the hosts of ~/.ssh/config, the Updates tab). The native box stays, unseen, for the
 * keyboard, the screen readers and the forms (ADR-0004). */
export function CheckboxControl({ checked, onChange, disabled = false }: CheckboxControlProps) {
  return (
    <span className={styles.control}>
      <input
        type="checkbox"
        checked={checked}
        disabled={disabled}
        className={styles.input}
        onChange={(event) => onChange(event.target.checked)}
      />
      <span className={styles.box} aria-hidden="true">
        <svg className={styles.check} width="14" height="14" viewBox="0 0 14 14" fill="none">
          <path d="M3 7.4 5.8 10 11 4.2" />
        </svg>
      </span>
    </span>
  );
}

/** A checkbox drawn by Vigie, the same in every engine, whose whole label can be clicked. */
export function Checkbox({ label, checked, onChange, note, hideLabel = false, disabled = false }: CheckboxProps) {
  const row = [styles.row, note && !hideLabel ? styles.withNote : ""].filter(Boolean).join(" ");
  return (
    <label className={row} data-disabled={disabled || undefined}>
      <CheckboxControl checked={checked} onChange={onChange} disabled={disabled} />
      {hideLabel ? (
        <span className={styles.hidden}>{label}</span>
      ) : note ? (
        <span className={styles.text}>
          <span className={styles.label}>{label}</span>
          <span className={styles.note}>{note}</span>
        </span>
      ) : (
        <span>{label}</span>
      )}
    </label>
  );
}
