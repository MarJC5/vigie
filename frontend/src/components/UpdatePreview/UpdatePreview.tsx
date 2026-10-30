import { useTranslation } from "react-i18next";

import type { SitePreviewView, UpdateItemView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { itemName } from "../../lib/updates";
import { Icon, type IconName } from "../Icon/Icon";
import { Notice } from "../Notice/Notice";
import styles from "./UpdatePreview.module.css";

export interface UpdatePreviewProps {
  preview: SitePreviewView;
  /** The items of the preview chosen, in the order of the run. */
  planned: UpdateItemView[];
  /** The titles of the plugins and themes, by key. */
  names: Map<string, string>;
}

/** What an update will change and what Vigie backs up first, or why it cannot start. */
export function UpdatePreview({ preview, planned, names }: UpdatePreviewProps) {
  const { t } = useTranslation();
  if (preview.problem) {
    const text = t(preview.problem.code, { message: preview.problem.detail });
    const detail = preview.problem.detail && !text.includes(preview.problem.detail) ? preview.problem.detail : undefined;
    return <Notice title={t("update.preview.cannot")} text={text} detail={detail} />;
  }
  if (!preview.dumpTool) return <Notice title={t("update.preview.cannot")} text={t("problem.noDumpTool")} />;
  if (planned.length === 0) return <Notice title={t("update.preview.cannot")} text={t("update.preview.nothing")} tone="warn" />;

  const plugins = planned.filter((it) => it.kind === "plugin").length;
  const themes = planned.filter((it) => it.kind === "theme").length;
  const backups: Array<[IconName, string]> = [["database", t("update.preview.database")]];
  if (plugins > 0) backups.push(["archive", t("update.preview.plugins", { count: plugins })]);
  if (themes > 0) backups.push(["archive", t("update.preview.themes", { count: themes })]);
  if (planned.some((it) => it.kind === "core")) backups.push(["archive", t("update.preview.coreFiles")]);
  const languages = planned.filter((it) => it.kind === "language");
  const items = planned.filter((it) => it.kind !== "language");

  return (
    <>
      <div className={styles.columns}>
        <section className={styles.column}>
          <h2 className={styles.heading}>
            {t("update.preview.whatChanges")}
          </h2>
          <ul className={styles.list}>
            {items.map((it) => (
              <li key={`${it.kind}-${it.slug}`} className={styles.change}>
                <span className={styles.line}>
                  <span className={styles.name}>{itemName(t, it.kind, it.slug, names)}</span>
                  <span className={styles.versions}>
                    {it.from} → {it.to}
                  </span>
                </span>
                <span className={styles.kind}>{t(`update.preview.kind.${it.kind}`)}</span>
              </li>
            ))}
            {languages.length > 0 && (
              <li className={styles.change}>
                <span className={styles.line}>
                  <span className={styles.name}>{t("update.preview.kind.language")}</span>
                  <span className={styles.versions}>{[...new Set(languages.map((it) => it.language))].join(", ")}</span>
                </span>
                <span className={styles.kind}>
                  {languages.map((it) => (it.slug ? itemName(t, "plugin", it.slug, names) : t("update.group.core"))).join(", ")}
                </span>
              </li>
            )}
          </ul>
        </section>
        <section className={styles.column}>
          <h2 className={styles.heading}>
            {t("update.preview.backupsFirst")}
          </h2>
          <ul className={styles.list}>
            {backups.map(([icon, label]) => (
              <li key={label} className={styles.backup}>
                <span className={styles.icon}>
                  <Icon name={icon} />
                </span>
                <span className={styles.name}>{label}</span>
              </li>
            ))}
          </ul>
          <p className={styles.where}>{t("update.preview.where")}</p>
          <p className={styles.path} data-selectable>
            {preview.backups}
          </p>
        </section>
      </div>
      {planned.some((it) => it.kind === "core") && (
        <Notice title={t("update.preview.coreTitle")} text={t("update.preview.coreText")} tone="warn" role="note" />
      )}
    </>
  );
}
