import { useTranslation } from "react-i18next";

import type { ChangeView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { asksForALook, changeText } from "../../lib/changes";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./ChangeList.module.css";

export interface ChangeListProps {
  changes: ChangeView[];
  /** The list's accessible name. */
  label: string;
}

/** What changed on a site since its previous complete scan (ADR-0027), a line each: what asks for
 a look with the mark of "needs attention", the rest in gray. The one way the window lists them. */
export function ChangeList({ changes, label }: ChangeListProps) {
  const { t, i18n } = useTranslation();
  return (
    <ul className={styles.list} aria-label={label}>
      {changes.map((c, i) => {
        const look = asksForALook(c);
        return (
          <li key={`${c.kind}-${c.subject}-${i}`} className={styles.change} data-quiet={look ? undefined : true}>
            <StatusDot status={look ? "warn" : "unknown"} label={t(look ? "changes.look" : "changes.change")} />
            <span>{changeText(c, t, i18n.language)}</span>
          </li>
        );
      })}
    </ul>
  );
}
