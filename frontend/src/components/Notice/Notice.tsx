import { Button } from "../Button/Button";
import styles from "./Notice.module.css";

export interface NoticeProps {
  title: string;
  text: string;
  /** The raw message (for example ssh's), smaller, for the curious or for support. */
  detail?: string;
  action?: { label: string; onClick: () => void };
}

/** A problem that stops a step, announced at once, with what to do and an action to try again. */
export function Notice({ title, text, detail, action }: NoticeProps) {
  return (
    <div role="alert" className={styles.notice}>
      <span className={styles.title}>{title}</span>
      <span>{text}</span>
      {detail && <span className={styles.detail}>{detail}</span>}
      {action && (
        <span>
          <Button onClick={action.onClick}>{action.label}</Button>
        </span>
      )}
    </div>
  );
}
