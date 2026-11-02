import { useTranslation } from "react-i18next";

import { Button } from "../Button/Button";
import { CommandLine } from "../CommandLine/CommandLine";
import { Dialog } from "../Dialog/Dialog";

export interface BackupsLeftDialogProps {
  /** The site or host deleted. */
  name: string;
  kind: "site" | "host";
  /** The command that removes the backups left, to run on the account. */
  command: string;
  onCopy?: (text: string) => Promise<unknown> | void;
  onClose: () => void;
}

/** Says that a site or host left Vigie while its server could not be reached: its backups stay
 there, and the command that removes them (ADR-0033). */
export function BackupsLeftDialog({ name, kind, command, onCopy, onClose }: BackupsLeftDialogProps) {
  const { t } = useTranslation();
  return (
    <Dialog
      title={t(kind === "host" ? "backups.left.titleHost" : "backups.left.titleSite", { name })}
      lead={t("backups.left.lead")}
      onClose={onClose}
      actions={
        <Button variant="primary" onClick={onClose}>
          {t("backups.left.close")}
        </Button>
      }
    >
      <CommandLine command={command} onCopy={onCopy} />
    </Dialog>
  );
}
