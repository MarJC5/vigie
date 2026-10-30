import styles from "./Switch.module.css";

export interface SwitchProps {
  /** The accessible name: what the switch turns on. */
  label: string;
  on: boolean;
  onChange: (on: boolean) => void;
  disabled?: boolean;
}

/** An on and off switch: a button with the switch role, its state read as checked. */
export function Switch({ label, on, onChange, disabled }: SwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={on}
      aria-label={label}
      disabled={disabled}
      className={styles.switch}
      onClick={() => onChange(!on)}
    >
      <span className={styles.knob} />
    </button>
  );
}
