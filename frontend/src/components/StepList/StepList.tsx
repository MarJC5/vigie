import { useEffect, useRef } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./StepList.module.css";

export type StepState = "done" | "running" | "failed" | "waiting";

export interface StepRow {
  key: string;
  state: StepState;
  label: string;
  detail: string;
  /** At the end of the row: how long it took, or where it stands. */
  end?: string;
}

export interface StepListProps {
  label: string;
  rows: StepRow[];
  /** What each state is called, for screen readers. */
  stateLabels: Record<StepState, string>;
  /** The run is over: the last step stays in sight. */
  ended?: boolean;
}

const icons: Record<StepState, IconName> = { done: "ok", running: "spinner", failed: "failed", waiting: "circle" };

/** The steps of a run as they go: done, under way, failed, or not reached; the one under way in sight. */
export function StepList({ label, rows, stateLabels, ended = false }: StepListProps) {
  const list = useRef<HTMLOListElement>(null);
  const running = rows.find((r) => r.state === "running")?.key;
  useEffect(() => {
    const items = list.current?.querySelectorAll<HTMLElement>("li");
    const target = running ? list.current?.querySelector<HTMLElement>("[aria-current=step]") : ended ? items?.[items.length - 1] : undefined;
    const still = window.matchMedia("(prefers-reduced-motion: reduce)").matches;
    target?.scrollIntoView({ block: "nearest", behavior: still ? "auto" : "smooth" });
  }, [running, ended]);
  return (
    <ol ref={list} aria-label={label} className={styles.steps}>
      {rows.map((row) => (
        <li key={row.key} className={styles.step} data-state={row.state} aria-current={row.state === "running" ? "step" : undefined}>
          <span className={styles.icon}>
            <Icon name={icons[row.state]} label={stateLabels[row.state]} />
          </span>
          <span className={styles.label}>{row.label}</span>
          <span className={styles.detail} data-selectable>
            {row.detail}
          </span>
          <span className={styles.time}>{row.end ?? ""}</span>
        </li>
      ))}
    </ol>
  );
}
