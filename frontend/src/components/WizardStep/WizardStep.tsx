import { type ReactNode, useId } from "react";

import { Button } from "../Button/Button";
import { LinkButton } from "../LinkButton/LinkButton";
import { ProgressBar, type ProgressProps } from "../Progress/Progress";
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
  /** A wait: its sentence under the content, its bar along the whole bottom of the window. */
  progress?: ProgressProps;
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
  progress,
  children,
}: WizardStepProps) {
  const progressId = useId();
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
          {progress && (
            <p id={progressId} className={styles.progress}>
              {progress.label}
            </p>
          )}
        </div>
      </main>
      <div className={styles.bar}>
        {progress && (
          <div className={styles.edge}>
            <ProgressBar labelledBy={progressId} value={progress.value} max={progress.max} placement="edge" />
          </div>
        )}
        <div className={styles.side}>
          {back && (
            <LinkButton onClick={back.onClick} disabled={back.disabled}>
              {back.label}
            </LinkButton>
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
