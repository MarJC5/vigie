import type { ReactNode } from "react";

import { Button } from "../Button/Button";
import { Stepper } from "../Stepper/Stepper";
import styles from "./WizardStep.module.css";

export interface WizardAction {
  label: string;
  onClick: () => void;
  disabled?: boolean;
  /** Waiting for an answer: disabled and announced as busy. */
  busy?: boolean;
}

export interface WizardStepProps {
  title: string;
  lead: string;
  steps: string[];
  current: number;
  stepsLabel: string;
  doneLabel: string;
  closeLabel: string;
  onClose: () => void;
  back?: WizardAction;
  primary: WizardAction;
  /** "wide" for tables, "narrow" (default) for forms and checks. */
  width?: "narrow" | "wide";
  children: ReactNode;
}

/** A full-window wizard step: centered title, content, and a bottom bar with the steps. */
export function WizardStep({
  title,
  lead,
  steps,
  current,
  stepsLabel,
  doneLabel,
  closeLabel,
  onClose,
  back,
  primary,
  width = "narrow",
  children,
}: WizardStepProps) {
  return (
    <div className={styles.window}>
      <div className={styles.top}>
        <Button variant="icon" icon="close" label={closeLabel} onClick={onClose} />
      </div>
      {/* Focusable, so a step taller than the window scrolls from the keyboard too. */}
      <main className={styles.main} tabIndex={0}>
        <div className={styles.content} data-width={width}>
          <div className={styles.heading}>
            <h1 className={styles.title}>{title}</h1>
            <p className={styles.lead}>{lead}</p>
          </div>
          {children}
        </div>
      </main>
      <div className={styles.bar}>
        <div className={styles.side}>
          {back && (
            <Button variant="text" onClick={back.onClick} disabled={back.disabled}>
              {back.label}
            </Button>
          )}
        </div>
        <Stepper label={stepsLabel} steps={steps} current={current} doneLabel={doneLabel} />
        <div className={`${styles.side} ${styles.end}`}>
          <Button
            variant="primary"
            onClick={primary.onClick}
            disabled={primary.disabled || primary.busy}
            aria-busy={primary.busy || undefined}
          >
            {primary.label}
          </Button>
        </div>
      </div>
    </div>
  );
}
