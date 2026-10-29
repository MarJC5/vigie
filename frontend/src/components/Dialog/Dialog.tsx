import { type KeyboardEvent, type ReactNode, useEffect, useId, useLayoutEffect, useRef } from "react";

import styles from "./Dialog.module.css";

export interface DialogProps {
  title: string;
  /** A sentence under the title, read as the dialog's description. */
  lead?: string;
  children?: ReactNode;
  /** The buttons at the bottom, the main one last. */
  actions: ReactNode;
  /** Escape, or the cancel button the caller puts in actions. */
  onClose: () => void;
  /** For forms: wider than a confirmation. */
  wide?: boolean;
}

const focusables = 'a[href], button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), [tabindex]:not([tabindex="-1"])';

// The dialogs open, the last on top: Escape closes that one only.
const open: Array<{ current: () => void }> = [];

/** A modal dialog over its positioned parent: the focus starts on its first field (else its first
 * button), stays inside with Tab, and goes back where it was when it closes. */
export function Dialog({ title, lead, children, actions, onClose, wide = false }: DialogProps) {
  const id = useId();
  const ref = useRef<HTMLDivElement>(null);

  // The latest onClose, for the Escape listener of the document.
  const close = useRef(onClose);
  useLayoutEffect(() => {
    close.current = onClose;
  }, [onClose]);

  useEffect(() => {
    const before = document.activeElement as HTMLElement | null;
    const dialog = ref.current;
    const first = dialog?.querySelector<HTMLElement>("input, select, textarea") ?? dialog?.querySelector<HTMLElement>(focusables);
    first?.focus();
    // On the document: WebKit does not focus a radio or a button when it is clicked, so the focus
    // can be on the body while the dialog is open.
    open.push(close);
    const onEscape = (event: globalThis.KeyboardEvent) => {
      if (event.key !== "Escape" || open[open.length - 1] !== close) return;
      event.preventDefault();
      close.current();
    };
    document.addEventListener("keydown", onEscape);
    return () => {
      document.removeEventListener("keydown", onEscape);
      open.splice(open.indexOf(close), 1);
      before?.focus();
    };
  }, []);

  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key !== "Tab") return;
    const items = [...(ref.current?.querySelectorAll<HTMLElement>(focusables) ?? [])];
    const first = items[0];
    const last = items[items.length - 1];
    if (event.shiftKey && document.activeElement === first) {
      event.preventDefault();
      last?.focus();
    } else if (!event.shiftKey && document.activeElement === last) {
      event.preventDefault();
      first?.focus();
    }
  };

  return (
    <div className={styles.backdrop}>
      <div
        ref={ref}
        role="dialog"
        aria-modal="true"
        aria-labelledby={`${id}-title`}
        aria-describedby={lead ? `${id}-lead` : undefined}
        className={wide ? `${styles.dialog} ${styles.wide}` : styles.dialog}
        onKeyDown={onKeyDown}
      >
        <div className={styles.head}>
          <h2 id={`${id}-title`} className={styles.title}>
            {title}
          </h2>
          {lead && (
            <p id={`${id}-lead`} className={styles.lead}>
              {lead}
            </p>
          )}
        </div>
        {children}
        <div className={styles.actions}>{actions}</div>
      </div>
    </div>
  );
}
