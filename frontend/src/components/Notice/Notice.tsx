import { Button } from "../Button/Button";
import { Icon, type IconName } from "../Icon/Icon";
import styles from "./Notice.module.css";

export interface NoticeProps {
  title: string;
  text: string;
  /** The raw message (for example ssh's), smaller, for the curious or for support. */
  detail?: string;
  action?: { label: string; onClick: () => void };
  /** danger (default) for a problem that stops; warn for what to know before going on; neutral for
   what to know that is no problem, in gray. */
  tone?: "danger" | "warn" | "neutral";
  /** The icon of a neutral notice. */
  icon?: IconName;
  /** alert (default) is announced at once; note is read in its place. */
  role?: "alert" | "note";
}

/** A problem that stops a step, announced at once, with what to do and an action to try again. */
export function Notice({ title, text, detail, action, tone = "danger", role = "alert", icon = "warning" }: NoticeProps) {
  return (
    <div role={role} className={styles.notice} data-tone={tone}>
      <span className={styles.icon} aria-hidden="true">
        <Icon name={tone === "neutral" ? icon : tone === "warn" ? "warning" : "failed"} size={16} />
      </span>
      <span className={styles.body}>
        <span className={styles.title}>{title}</span>
        <span>{text}</span>
        {detail && (
          <span className={styles.detail} data-selectable>
            {detail}
          </span>
        )}
        {action && (
          <span>
            <Button className={styles.action} onClick={action.onClick}>
              {action.label}
            </Button>
          </span>
        )}
      </span>
    </div>
  );
}
