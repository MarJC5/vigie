import { useTranslation } from "react-i18next";

import type { ImportPreview, Node, Problem } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { type ImportGroup, folderOptions, groupsOf, itemDetail, itemIcon, itemName } from "../../lib/data";
import { clock, day } from "../../lib/findings";
import { Button } from "../Button/Button";
import { CheckboxControl } from "../Checkbox/Checkbox";
import { Icon } from "../Icon/Icon";
import { LinkButton } from "../LinkButton/LinkButton";
import { Notice } from "../Notice/Notice";
import { SelectField } from "../SelectField/SelectField";
import styles from "./ImportWindow.module.css";

export interface ImportWindowProps {
  preview: ImportPreview;
  /** The site manager, for the folder the import goes into. */
  nodes: Node[];
  onInto: (folderId: string) => void;
  /** The hosts whose key is kept. None is at first: a file can bring a key never checked. */
  keep: Set<string>;
  onKeep: (hostId: string, on: boolean) => void;
  onConfirm: () => void;
  onClose: () => void;
  /** The import is being added, or the preview read again. */
  busy?: boolean;
  problem?: Problem | null;
}

const order: ImportGroup[] = ["added", "present", "refused"];

/** The preview of an import: what is added, what is already there and left as it is, what is
 refused, and the server keys the file brings, each kept only if the user checks it. Nothing is
 added before the confirmation. */
export function ImportWindow({ preview, nodes, onInto, keep, onKeep, onConfirm, onClose, busy = false, problem }: ImportWindowProps) {
  const { t } = useTranslation();
  const groups = groupsOf(preview.items ?? []);
  const keys = preview.keys ?? [];
  const addon = preview.source === "addon";
  const lead = addon
    ? t("data.preview.leadAddOn", { addon: `vigie-${preview.addOn}`, release: preview.app })
    : preview.source === "csv"
      ? t("data.preview.leadCSV", { count: preview.lines, separator: t(`data.separator.${preview.separator === ";" ? "semicolon" : "comma"}`) })
      : t(preview.history ? "data.preview.leadHistory" : "data.preview.lead", {
          date: day(preview.exported),
          time: clock(preview.exported),
          app: preview.app,
        });
  return (
    <div className={styles.window}>
      <div className={styles.top}>
        <Button variant="icon" icon="close" label={t("wizard.close")} onClick={onClose} />
      </div>
      <main className={styles.main} tabIndex={0}>
        <div className={styles.content}>
          <div className={styles.heading}>
            <h1 className={styles.title}>
              {addon ? t("data.preview.titleAddOn", { title: preview.addOnTitle }) : t("data.preview.title", { file: preview.file })}
            </h1>
            <p className={styles.lead}>{lead}</p>
          </div>
          <div className={styles.into}>
            <SelectField label={t("data.into")} value={preview.into} options={folderOptions(t, nodes)} onChange={onInto} />
          </div>
          {problem && <Notice title={t("data.preview.failed")} text={t(problem.code, { message: problem.detail })} />}
          <div className={styles.columns} data-keys={keys.length > 0 || (addon && preview.report) ? true : undefined}>
            <div className={styles.groups}>
              {order.map(
                (g) =>
                  groups[g].length > 0 && (
                    <section key={g} className={styles.group}>
                      <h2 className={styles.groupTitle}>
                        {t(`data.preview.${g}`)}
                        <span className={styles.count}>{groups[g].length}</span>
                      </h2>
                      <ul className={styles.list}>
                        {groups[g].map((item, i) => (
                          <li key={`${item.kind}-${item.path}-${item.line}-${i}`} className={styles.item} data-muted={g !== "added" || undefined}>
                            <Icon name={itemIcon(item)} />
                            <span className={styles.itemText}>
                              <span className={styles.name}>{itemName(t, item)}</span>
                              <span className={styles.detail} data-selectable={item.kind === "line" || undefined}>
                                {itemDetail(t, item)}
                              </span>
                            </span>
                          </li>
                        ))}
                      </ul>
                    </section>
                  ),
              )}
            </div>
            {(keys.length > 0 || (addon && preview.report)) && (
              <div className={styles.side}>
                {addon && preview.report && (
                  <section className={styles.keys}>
                    <h2 className={styles.groupTitle}>{t("data.preview.report")}</h2>
                    <p className={styles.keysLead}>{t("data.preview.reportLead", { addon: `vigie-${preview.addOn}` })}</p>
                    <pre className={styles.report} data-selectable>
                      {preview.report}
                    </pre>
                  </section>
                )}
                {keys.length > 0 && (
                  <section className={styles.keys}>
                    <h2 className={styles.groupTitle}>{t("data.preview.keys")}</h2>
                    <p className={styles.keysLead}>{t("data.preview.keysLead")}</p>
                    {keys.map((k) =>
                      k.refused ? (
                        <div key={k.hostId + k.fingerprint} className={styles.key} data-refused>
                          <span className={styles.itemText}>
                            <span className={styles.name}>{k.host}</span>
                            <span className={styles.detail}>{t("data.preview.keyRefused", { reason: t(`cli.import.key.${k.refused}`) })}</span>
                          </span>
                        </div>
                      ) : (
                        <label key={k.hostId + k.fingerprint} className={styles.key}>
                          <CheckboxControl checked={keep.has(k.hostId)} onChange={(checked) => onKeep(k.hostId, checked)} />
                          <span className={styles.itemText}>
                            <span className={styles.name}>
                              {k.host} · {k.server}
                            </span>
                            <span className={styles.fingerprint} data-selectable>
                              {k.type} {k.fingerprint}
                            </span>
                          </span>
                        </label>
                      ),
                    )}
                  </section>
                )}
              </div>
            )}
          </div>
        </div>
      </main>
      <div className={styles.bar}>
        <LinkButton onClick={onClose}>
          {t("action.cancel")}
        </LinkButton>
        <span className={styles.later}>{preview.added ? t("data.preview.nothingYet") : t("data.preview.nothingNew")}</span>
        <span className={styles.actions}>
          <Button variant="primary" disabled={busy || preview.added === 0} aria-busy={busy} onClick={onConfirm}>
            {t("data.preview.confirm", { count: preview.added })}
          </Button>
        </span>
      </div>
    </div>
  );
}
