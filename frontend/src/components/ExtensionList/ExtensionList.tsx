import { Fragment, useMemo, useState } from "react";
import { useTranslation } from "react-i18next";

import type { ExtensionView, SubsiteView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "../../lib/findings";
import { activation, labelsOf } from "../../lib/subsites";
import { ignoredText } from "../../lib/updates";
import { FilterPad } from "../FilterPad/FilterPad";
import { Icon } from "../Icon/Icon";
import { LinkButton } from "../LinkButton/LinkButton";
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
  /** A network's subsites (ADR-0029): each plugin and theme then says where it is active. */
  subsites?: SubsiteView[];
  /** Opens a subsite's page. */
  onOpenSubsite?: (blogId: number) => void;
  /** Without the filters and the groups' titles, for a short list under a title of its own (a
   * subsite's plugins). */
  compact?: boolean;
}

// What each filter keeps; a must-use plugin or a drop-in always runs, a parent theme too. On a
// network, active is active anywhere on it.
const inactive = (e: ExtensionView) => activation(e)?.kind === "nowhere" || (activation(e) === undefined && e.status === "inactive");
const keeps: Record<ExtensionFilter, (e: ExtensionView) => boolean> = {
  all: () => true,
  active: (e) => !inactive(e),
  inactive,
  update: (e) => e.update === "available",
};

/** The address of a plugin or theme on wordpress.org. */
export function wordPressOrgAddress(e: { kind: string; slug: string }): string {
  return `https://wordpress.org/${e.kind === "theme" ? "themes" : "plugins"}/${e.slug}/`;
}

/** The plugins, then the themes, of a site's latest scan, with a filter (ADR-0024, point 1). */
export function ExtensionList({ extensions, onFinding, onOpen, subsites, onOpenSubsite, compact = false }: ExtensionListProps) {
  const { t, i18n } = useTranslation();
  const [filter, setFilter] = useState<ExtensionFilter>("all");
  // The plugin or theme whose subsites are listed.
  const [opened, setOpened] = useState<string>();
  const network = extensions.some((e) => activation(e) !== undefined);
  const labels = useMemo(() => labelsOf(subsites), [subsites]);
  const labelOf = (blogId: number) => labels[String(blogId)] ?? String(blogId);
  const shown = extensions.filter(keeps[filter]);
  const groups = [
    { kind: "plugin", title: t("ext.plugins"), items: shown.filter((e) => e.kind === "plugin") },
    { kind: "theme", title: t("ext.themes"), items: shown.filter((e) => e.kind === "theme") },
  ].filter((g) => g.items.length > 0);

  return (
    <div className={styles.list}>
      {!compact && (
        <div role="group" aria-label={t("ext.filter")} className={styles.filters}>
          {(["all", "active", "inactive", "update"] as const).map((f) => (
            <FilterPad key={f} label={t(`ext.filters.${f}`)} count={extensions.filter(keeps[f]).length} pressed={filter === f} onSelect={() => setFilter(f)} />
          ))}
        </div>
      )}
      {groups.length === 0 && <p className={styles.empty}>{t("ext.none")}</p>}
      {groups.map((g) => (
        <section key={g.kind} className={styles.group}>
          {!compact && <h2 className={styles.title}>{g.title}</h2>}
          <table className={styles.table} aria-label={g.title} data-network={network || undefined}>
            <thead>
              <tr>
                <th scope="col">{t("ext.col.name")}</th>
                <th scope="col">{t("ext.col.version")}</th>
                <th scope="col">{t(network ? "ext.col.activation" : "ext.col.status")}</th>
                <th scope="col">{t("ext.col.update")}</th>
                <th scope="col">{t("ext.col.auto")}</th>
                <th scope="col">{t("ext.col.finding")}</th>
              </tr>
            </thead>
            <tbody>
              {g.items.map((e) => {
                const where = activation(e);
                const key = `${e.kind}:${e.slug}`;
                return (
                  <Fragment key={key}>
                    <tr>
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
                      {where?.kind === "some" ? (
                        <td>
                          <LinkButton tone="text" size="body" aria-expanded={opened === key} onClick={() => setOpened(opened === key ? undefined : key)}>
                            <Icon name={opened === key ? "caretDown" : "caretRight"} size="xs" />
                            {t("subsite.activation.some", { count: where.sites.length })}
                          </LinkButton>
                        </td>
                      ) : where ? (
                        <td className={where.kind === "nowhere" ? styles.muted : undefined}>{t(`subsite.activation.${where.kind}`)}</td>
                      ) : (
                        <td className={styles.muted}>{t(`ext.status.${e.kind}.${e.status}`, { defaultValue: e.status })}</td>
                      )}
                      <td>
                        {e.update === "available" && e.ignore ? (
                          // Ignored (ADR-0026): said, in gray, without the mark of an update waiting.
                          <span className={`${styles.update} ${styles.muted}`} title={ignoredText(e.ignore, t)}>
                            <Icon name="eyeSlash" size="sm" />
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
                          <LinkButton onClick={onFinding} title={findingText(e.worst, t, i18n.language).title}>
                            <StatusDot status={e.worst.level === "urgent" ? "danger" : "warn"} />
                            <span className={styles.ellipsis}>{findingText(e.worst, t, i18n.language).title}</span>
                          </LinkButton>
                        ) : (
                          !e.onWordPressOrg &&
                          e.kind === "plugin" &&
                          e.status !== "must-use" &&
                          e.status !== "dropin" && <span className={styles.muted}>{t("ext.notOrg")}</span>
                        )}
                      </td>
                    </tr>
                    {where?.kind === "some" && opened === key && (
                      <tr>
                        <td colSpan={6} className={styles.sites}>
                          <ul aria-label={t("subsite.activation.listOf", { name: e.title || e.slug })}>
                            {where.sites.map((id) => (
                              <li key={id}>
                                {onOpenSubsite ? (
                                  <LinkButton onClick={() => onOpenSubsite(id)}>{labelOf(id)}</LinkButton>
                                ) : (
                                  labelOf(id)
                                )}
                              </li>
                            ))}
                          </ul>
                        </td>
                      </tr>
                    )}
                  </Fragment>
                );
              })}
            </tbody>
          </table>
        </section>
      ))}
    </div>
  );
}
