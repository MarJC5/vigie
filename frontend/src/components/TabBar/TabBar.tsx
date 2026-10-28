import { type KeyboardEvent, useRef } from "react";

import styles from "./TabBar.module.css";

export interface Tab {
  id: string;
  label: string;
  /** A count shown in a badge, for example pending updates. */
  count?: number;
}

export interface TabBarProps {
  label: string;
  tabs: Tab[];
  value: string;
  onChange: (id: string) => void;
}

/** Tabs with the WAI-ARIA keyboard pattern: arrows, Home and End move and select. */
export function TabBar({ label, tabs, value, onChange }: TabBarProps) {
  const refs = useRef<Array<HTMLButtonElement | null>>([]);

  const move = (event: KeyboardEvent, index: number) => {
    const last = tabs.length - 1;
    const next = { ArrowRight: index === last ? 0 : index + 1, ArrowLeft: index === 0 ? last : index - 1, Home: 0, End: last }[
      event.key
    ];
    if (next === undefined) return;
    event.preventDefault();
    onChange(tabs[next]!.id);
    refs.current[next]?.focus();
  };

  return (
    <div role="tablist" aria-label={label} className={styles.bar}>
      {tabs.map((tab, index) => {
        const selected = tab.id === value;
        return (
          <button
            key={tab.id}
            ref={(el) => {
              refs.current[index] = el;
            }}
            type="button"
            role="tab"
            aria-selected={selected}
            tabIndex={selected ? 0 : -1}
            className={styles.tab}
            onClick={() => onChange(tab.id)}
            onKeyDown={(event) => move(event, index)}
          >
            {tab.label}
            {tab.count !== undefined && <span className={styles.count}>{tab.count}</span>}
          </button>
        );
      })}
    </div>
  );
}
