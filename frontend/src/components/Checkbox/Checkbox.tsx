import styles from "./Checkbox.module.css";

export interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  /** A sentence under the label, saying what the choice means. */
  note?: string;
}

/** A checkbox whose whole label can be clicked. */
export function Checkbox({ label, checked, onChange, note }: CheckboxProps) {
  return (
    <label className={note ? `${styles.row} ${styles.withNote}` : styles.row}>
      <input type="checkbox" checked={checked} className={styles.box} onChange={(event) => onChange(event.target.checked)} />
      {note ? (
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
