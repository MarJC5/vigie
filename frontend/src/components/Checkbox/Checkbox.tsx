import styles from "./Checkbox.module.css";

export interface CheckboxProps {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}

/** A checkbox whose whole label can be clicked. */
export function Checkbox({ label, checked, onChange }: CheckboxProps) {
  return (
    <label className={styles.row}>
      <input type="checkbox" checked={checked} className={styles.box} onChange={(event) => onChange(event.target.checked)} />
      <span>{label}</span>
    </label>
  );
}
