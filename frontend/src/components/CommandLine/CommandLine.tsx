import { useEffect, useState } from "react";
import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import styles from "./CommandLine.module.css";

export interface CommandLineProps {
  command: string;
  /** Puts the command on the clipboard; without it, the command is only shown, selectable. */
  onCopy?: (text: string) => Promise<unknown> | void;
}

/** A shell command to run by hand, selectable, with a button that copies it. */
export function CommandLine({ command, onCopy }: CommandLineProps) {
  const { t } = useTranslation();
  // "Copied" for two seconds after the command was copied.
  const [copied, setCopied] = useState(false);
  useEffect(() => {
    if (!copied) return;
    const timer = setTimeout(() => setCopied(false), 2000);
    return () => clearTimeout(timer);
  }, [copied]);
  return (
    <div className={styles.command}>
      <code className={styles.text} data-selectable>
        {command}
      </code>
      {onCopy && (
        <Button
          variant="icon"
          icon={copied ? "check" : "copy"}
          label={t(copied ? "backups.copied" : "backups.copy")}
          onClick={() => void Promise.resolve(onCopy(command)).then(() => setCopied(true))}
        />
      )}
    </div>
  );
}
