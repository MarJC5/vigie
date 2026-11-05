import styles from "./Radio.module.css";

export interface RadioControlProps {
  /** The group's name: the radios of one group share it, and the arrow keys move within it. */
  name: string;
  checked: boolean;
  onChange: () => void;
  disabled?: boolean;
}

/** The drawn dot alone, for a row whose own <label> holds it with several cells (the destinations
 * of a move). The native radio stays, unseen, for the keyboard, the screen readers and the forms
 * (ADR-0004). */
export function RadioControl({ name, checked, onChange, disabled = false }: RadioControlProps) {
  return (
    <span className={styles.control}>
      <input type="radio" name={name} checked={checked} disabled={disabled} className={styles.input} onChange={onChange} />
      <span className={styles.dot} aria-hidden="true" />
    </span>
  );
}

export interface RadioProps extends RadioControlProps {
  label: string;
}

/** One choice of a list without descriptions, drawn by Vigie, whose whole label can be clicked. A
 * choice with a description is a ChoiceCard. */
export function Radio({ label, name, checked, onChange, disabled = false }: RadioProps) {
  return (
    <label className={styles.row} data-disabled={disabled || undefined}>
      <RadioControl name={name} checked={checked} onChange={onChange} disabled={disabled} />
      <span>{label}</span>
    </label>
  );
}
