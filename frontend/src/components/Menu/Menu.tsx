import { type CSSProperties, type KeyboardEvent, useEffect, useLayoutEffect, useRef, useState } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import type { SiteColor } from "../TreeNode/TreeNode";
import styles from "./Menu.module.css";

export type ColorChoice = "none" | SiteColor;

const colors: ColorChoice[] = ["none", "red", "green", "blue", "yellow", "cyan", "magenta", "orange"];

export type MenuEntry =
  | {
      kind: "item";
      id: string;
      label: string;
      icon: IconName;
      /** Shown on the right, for example "⌘R"; keyshortcuts announces it ("Meta+R"). */
      shortcut?: string;
      keyshortcuts?: string;
      /** Removes something: its icon takes the danger color. */
      danger?: boolean;
      disabled?: boolean;
    }
  | { kind: "separator" }
  /** A line that says what the menu acts on, for example "3 items selected". */
  | { kind: "heading"; label: string }
  /** value: the color of what the menu acts on; none checked when they differ. */
  | { kind: "colors"; label: string; value?: ColorChoice; names: Record<ColorChoice, string> };

export interface MenuProps {
  label: string;
  entries: MenuEntry[];
  /** Where it opens, in the coordinates of its positioned parent; it stays inside that parent. */
  x: number;
  y: number;
  onSelect: (id: string) => void;
  onColor?: (color: ColorChoice) => void;
  /** Escape, Tab, or a click outside. */
  onClose: () => void;
}

/** A menu of actions, opened at a point: arrow keys, Home and End move, Escape closes. */
export function Menu({ label, entries, x, y, onSelect, onColor, onClose }: MenuProps) {
  const ref = useRef<HTMLDivElement>(null);
  const [position, setPosition] = useState({ left: x, top: y });

  // Kept inside the parent: a menu opened near an edge moves back in.
  useLayoutEffect(() => {
    const menu = ref.current;
    const parent = menu?.offsetParent as HTMLElement | null;
    if (!menu || !parent) return;
    const left = Math.max(0, Math.min(x, parent.clientWidth - menu.offsetWidth - 8));
    const top = Math.max(0, Math.min(y, parent.clientHeight - menu.offsetHeight - 8));
    setPosition({ left, top });
  }, [x, y]);

  // The latest onClose, so a new function from the parent neither refocuses nor rebinds.
  const close = useRef(onClose);
  useLayoutEffect(() => {
    close.current = onClose;
  }, [onClose]);
  useEffect(() => {
    focusable(ref.current)[0]?.focus();
    const outside = (event: PointerEvent) => {
      if (!ref.current?.contains(event.target as Node)) close.current();
    };
    document.addEventListener("pointerdown", outside);
    return () => document.removeEventListener("pointerdown", outside);
  }, []);

  const onKeyDown = (event: KeyboardEvent) => {
    const items = focusable(ref.current);
    const index = items.indexOf(document.activeElement as HTMLElement);
    const go = (i: number) => {
      event.preventDefault();
      items[(i + items.length) % items.length]?.focus();
    };
    switch (event.key) {
      case "ArrowDown":
      case "ArrowRight":
        return go(index + 1);
      case "ArrowUp":
      case "ArrowLeft":
        return go(index - 1);
      case "Home":
        return go(0);
      case "End":
        return go(items.length - 1);
      case "Escape":
      case "Tab":
        event.preventDefault();
        return onClose();
    }
  };

  return (
    <div
      ref={ref}
      role="menu"
      aria-label={label}
      className={styles.menu}
      style={{ left: position.left, top: position.top }}
      onKeyDown={onKeyDown}
    >
      {entries.map((entry, i) => {
        if (entry.kind === "separator") return <div key={i} role="separator" className={styles.separator} />;
        if (entry.kind === "heading") {
          return (
            <div key={i} role="presentation" className={styles.heading}>
              {entry.label}
            </div>
          );
        }
        if (entry.kind === "colors") {
          return (
            <div key={i} role="group" aria-label={entry.label} className={styles.colors}>
              <Icon name="palette" />
              {colors.map((c) => (
                <button
                  key={c}
                  type="button"
                  role="menuitemradio"
                  aria-checked={entry.value === c}
                  aria-label={entry.names[c]}
                  tabIndex={-1}
                  className={styles.swatch}
                  style={{ "--vg-swatch": c === "none" ? "transparent" : `var(--vg-site-${c})` } as CSSProperties}
                  data-none={c === "none" || undefined}
                  onClick={() => onColor?.(c)}
                />
              ))}
            </div>
          );
        }
        return (
          <button
            key={entry.id}
            type="button"
            role="menuitem"
            aria-disabled={entry.disabled || undefined}
            aria-keyshortcuts={entry.keyshortcuts}
            tabIndex={-1}
            className={styles.item}
            onClick={() => !entry.disabled && onSelect(entry.id)}
          >
            <span className={entry.danger ? styles.danger : styles.icon}>
              <Icon name={entry.icon} />
            </span>
            <span className={styles.label}>{entry.label}</span>
            {entry.shortcut && (
              <span className={styles.shortcut} aria-hidden="true">
                {entry.shortcut}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}

// The items the keyboard reaches, in order: enabled items and the swatches.
function focusable(menu: HTMLElement | null): HTMLElement[] {
  return [...(menu?.querySelectorAll<HTMLElement>('[role=menuitem]:not([aria-disabled="true"]), [role=menuitemradio]') ?? [])];
}
