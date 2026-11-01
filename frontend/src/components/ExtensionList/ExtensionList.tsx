import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { ExtensionView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "../../lib/findings";
import { ignoredText } from "../../lib/updates";
import { Icon } from "../Icon/Icon";
import { StatusDot } from "../StatusDot/StatusDot";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import styles from "./ExtensionList.module.css";

export type ExtensionFilter = "all" | "active" | "inactive" | "update";

export interface ExtensionListProps {
  extensions: ExtensionView[];
  /** Opens the Security tab, from a plugin's or theme's worst finding. */
  onFinding?: () => void;
  /** Opens an address in the browser: a plugin's or theme's page on wordpress.org. */
  onOpen?: (href: string) => void;
}

// What each filter keeps; a must-use plugin or a drop-in always runs, a parent theme too.
const keeps: Record<ExtensionFilter, (e: ExtensionView) => boolean> = {
  all: () => true,
  active: (e) => e.status !== "inactive",
  inactive: (e) => e.status === "inactive",
  update: (e) => e.update === "available",
};

/** The address of a plugin or theme on wordpress.org. */
export function wordPressOrgAddress(e: { kind: string; slug: string }): string {
  return `https://wordpress.org/${e.kind === "theme" ? "themes" : "plugins"}/${e.slug}/`;
}

/** The plugins, then the themes, of a site's latest scan, with a filter (ADR-0024, point 1). */
export function ExtensionList({ extensions, onFinding, onOpen }: ExtensionListProps) {
  const { t, i18n } = useTranslation();
  const [filter, setFilter] = useState<ExtensionFilter>("all");
  const shown = extensions.filter(keeps[filter]);
  const groups = [
    { kind: "plugin", title: t("ext.plugins"), items: shown.filter((e) => e.kind === "plugin") },
    { kind: "theme", title: t("ext.themes"), items: shown.filter((e) => e.kind === "theme") },
  ].filter((g) => g.items.length > 0);

  return (
    <div className={styles.list}>
      <div role="group" aria-label={t("ext.filter")} className={styles.filters}>
        {(["all", "active", "inactive", "update"] as const).map((f) => (
          <button key={f} type="button" className={styles.filter} aria-pressed={filter === f} onClick={() => setFilter(f)}>
            {t(`ext.filters.${f}`)}
            <span className={styles.count}>{extensions.filter(keeps[f]).length}</span>
          </button>
        ))}
      </div>
      {groups.length === 0 && <p className={styles.empty}>{t("ext.none")}</p>}
      {groups.map((g) => (
        <section key={g.kind} className={styles.group}>
          <h2 className={styles.title}>{g.title}</h2>
          <table className={styles.table} aria-label={g.title}>
            <thead>
              <tr>
                <th scope="col">{t("ext.col.name")}</th>
                <th scope="col">{t("ext.col.version")}</th>
                <th scope="col">{t("ext.col.status")}</th>
                <th scope="col">{t("ext.col.update")}</th>
                <th scope="col">{t("ext.col.auto")}</th>
                <th scope="col">{t("ext.col.finding")}</th>
              </tr>
            </thead>
            <tbody>
              {g.items.map((e) => (
                <tr key={`${e.kind}:${e.slug}`}>
                  <td>
                    <span className={styles.name}>
                      {e.onWordPressOrg && onOpen ? (
                        <ExternalLink href={wordPressOrgAddress(e)} onOpen={onOpen} tone="text" size="body">
                          {e.title || e.slug}
                        </ExternalLink>
                      ) : (
                        <span className={styles.strong}>{e.title || e.slug}</span>
                      )}
                      <span className={styles.slug} data-selectable>
                        {e.slug}
                      </span>
                    </span>
                  </td>
                  <td className={styles.version} data-selectable>
                    {e.version}
                  </td>
                  <td className={styles.muted}>{t(`ext.status.${e.kind}.${e.status}`, { defaultValue: e.status })}</td>
                  <td>
                    {e.update === "available" && e.ignore ? (
                      // Ignored (ADR-0026): said, in gray, without the mark of an update waiting.
                      <span className={`${styles.update} ${styles.muted}`} title={ignoredText(e.ignore, t)}>
                        <Icon name="eyeSlash" size={14} />
                        {t("ext.to", { version: e.updateVersion })} · {t("ext.ignored")}
                      </span>
                    ) : e.update === "available" ? (
                      <span className={styles.update}>
                        <StatusDot status="warn" />
                        {t("ext.to", { version: e.updateVersion })}
                      </span>
                    ) : e.update === "none" ? (
                      <span className={styles.muted}>{t("overview.upToDate")}</span>
                    ) : (
                      <span className={styles.muted}>{t(`ext.updateState.${e.update}`)}</span>
                    )}
                  </td>
                  <td className={styles.muted}>{e.autoUpdate ? t("ext.auto") : ""}</td>
                  <td className={styles.finding}>
                    {e.worst ? (
                      <button type="button" className={styles.findingLink} onClick={onFinding} title={findingText(e.worst, t, i18n.language).title}>
                        <StatusDot status={e.worst.level === "urgent" ? "danger" : "warn"} />
                        <span className={styles.ellipsis}>{findingText(e.worst, t, i18n.language).title}</span>
                      </button>
                    ) : (
                      !e.onWordPressOrg &&
                      e.kind === "plugin" &&
                      e.status !== "must-use" &&
                      e.status !== "dropin" && <span className={styles.muted}>{t("ext.notOrg")}</span>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </section>
      ))}
    </div>
  );
}
