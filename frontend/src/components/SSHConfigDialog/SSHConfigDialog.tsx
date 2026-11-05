import { useTranslation } from "react-i18next";

import type { Node, Problem, SSHConfigView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { folderOptions } from "../../lib/data";
import { Button } from "../Button/Button";
import { CheckboxControl } from "../Checkbox/Checkbox";
import { Dialog } from "../Dialog/Dialog";
import { Notice } from "../Notice/Notice";
import { SelectField } from "../SelectField/SelectField";
import styles from "./SSHConfigDialog.module.css";

export interface SSHConfigDialogProps {
  /** The names of ~/.ssh/config; null while ssh reads them. */
  config: SSHConfigView | null;
  /** The site manager, for the folder they go into. */
  nodes: Node[];
  into: string;
  onInto: (folderId: string) => void;
  /** The aliases checked. None is at first: the file names every server, not only the ones of
   WordPress sites. */
  chosen: Set<string>;
  onToggle: (alias: string, on: boolean) => void;
  onImport: () => void;
  onClose: () => void;
  busy?: boolean;
  problem?: Problem | null;
}

/** Choose hosts of ~/.ssh/config to add: each with the account ssh reaches, the ones already in
 Vigie or that ssh cannot read shown but not offered. */
export function SSHConfigDialog({ config, nodes, into, onInto, chosen, onToggle, onImport, onClose, busy = false, problem }: SSHConfigDialogProps) {
  const { t } = useTranslation();
  const hosts = config?.hosts ?? [];
  return (
    <Dialog
      title={t("data.ssh.title")}
      lead={t("data.ssh.lead")}
      onClose={onClose}
      wide
      progress={config ? undefined : { label: t("data.ssh.loading") }}
      actions={
        <>
          <Button variant="outline" onClick={onClose}>
            {t("action.cancel")}
          </Button>
          <Button variant="primary" disabled={busy || chosen.size === 0} aria-busy={busy} onClick={onImport}>
            {chosen.size ? t("data.ssh.confirm", { count: chosen.size }) : t("data.ssh.confirmNone")}
          </Button>
        </>
      }
    >
      {problem && <Notice title={t("data.ssh.failed")} text={t(problem.code, { message: problem.detail })} />}
      {config && hosts.length === 0 && !problem && <p className={styles.note}>{t("data.ssh.none")}</p>}
      {hosts.length > 0 && (
        <div role="group" aria-label={t("data.ssh.title")} className={styles.list}>
          <div className={styles.head} aria-hidden="true">
            <span />
            <span>{t("data.ssh.alias")}</span>
            <span>{t("data.ssh.account")}</span>
            <span />
          </div>
          {hosts.map((h) => {
            const offered = !h.present && !h.error;
            return (
              <label key={h.alias} className={styles.row} data-off={offered ? undefined : true}>
                <CheckboxControl
                  checked={offered && chosen.has(h.alias)}
                  disabled={!offered}
                  onChange={(checked) => onToggle(h.alias, checked)}
                />
                <span className={styles.alias}>{h.alias}</span>
                <span className={styles.account} data-selectable>
                  {h.account}
                </span>
                <span className={styles.state}>{h.present ? t("data.ssh.present") : h.error ? t("data.ssh.error") : ""}</span>
              </label>
            );
          })}
        </div>
      )}
      {config && config.patterns > 0 && <p className={styles.note}>{t("data.ssh.patterns", { count: config.patterns })}</p>}
      {hosts.length > 0 && <SelectField label={t("data.into")} value={into} options={folderOptions(t, nodes)} onChange={onInto} />}
    </Dialog>
  );
}
