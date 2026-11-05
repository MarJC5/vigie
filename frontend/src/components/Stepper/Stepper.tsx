import { Icon } from "../Icon/Icon";
import styles from "./Stepper.module.css";

export interface StepperProps {
  label: string;
  steps: string[];
  /** Index of the current step. */
  current: number;
  /** Read by screen readers after a finished step, which shows a check mark. */
  doneLabel: string;
}

/** The numbered steps of a wizard: finished ones checked, the current one filled. */
export function Stepper({ label, steps, current, doneLabel }: StepperProps) {
  return (
    <ol aria-label={label} className={styles.steps}>
      {steps.map((step, index) => {
        const state = index < current ? "done" : index === current ? "current" : "next";
        return (
          <li key={step} className={styles.step} data-state={state} aria-current={state === "current" ? "step" : undefined}>
            <span className={styles.badge} aria-hidden="true">
              {state === "done" ? <Icon name="check" size="xs" /> : index + 1}
            </span>
            {step}
            {state === "done" && <span className={styles.hidden}>{doneLabel}</span>}
          </li>
        );
      })}
    </ol>
  );
}
