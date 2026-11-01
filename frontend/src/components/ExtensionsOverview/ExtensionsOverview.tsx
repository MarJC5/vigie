import { useState } from "react";
import { useTranslation } from "react-i18next";

import type { ExtensionsAcross } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { findingText } from "../../lib/findings";
import { wordPressOrgAddress } from "../ExtensionList/ExtensionList";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import { Icon } from "../Icon/Icon";
import { SearchField } from "../SearchField/SearchField";
import { StatusDot } from "../StatusDot/StatusDot";
import styles from "./ExtensionsOverview.module.css";

export interface ExtensionsOverviewProps {
  across: ExtensionsAcross;
  /** Opens a site's page. */
  onSelect: (siteId: string) => void;
  /** Opens an address in the browser: a plugin's or theme's page on wordpress.org. */
  onOpen?: (href: string) => void;
}

function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
}

/** The plugins and themes across a host's or folder's sites (ADR-0024, point 2): how many sites run
 * each, in which versions; a line opens to its sites. */
export function ExtensionsOverview({ across, onSelect, onOpen }: ExtensionsOverviewProps) {
  const { t, i18n } = useTranslation();
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState<Set<string>>(new Set());
  const q = fold(query.trim());
  const items = (across.items ?? []).filter((e) => !q || fold(e.title).includes(q) || fold(e.slug).includes(q));
  const toggle = (key: string) =>
    setOpen((current) => {
      const next = new Set(current);
      if (!next.delete(key)) next.add(key);
      return next;
    });
  const leftOut = across.leftOut ?? [];

  return (
    <div className={styles.overview}>
      <div className={styles.bar}>
        <div className={styles.search}>
          <SearchField label={t("ext.across.search")} value={query} onChange={setQuery} onClose={() => setQuery("")} />
        </div>
        {leftOut.length > 0 && (
          <p className={styles.leftOut}>
            {t("ext.across.leftOut", { count: leftOut.length, names: leftOut.map((s) => s.name).join(", ") })}
          </p>
        )}
      </div>
      {items.length === 0 ? (
        <p className={styles.empty}>{q ? t("ext.across.noMatch") : t("ext.across.none")}</p>
      ) : (
        <table className={styles.table} aria-label={t("ext.across.table")}>
          <thead>
            <tr>
              <th scope="col">
                <span className={styles.hidden}>{t("ext.across.open")}</span>
              </th>
              <th scope="col">{t("ext.col.name")}</th>
              <th scope="col">{t("ext.across.sites")}</th>
              <th scope="col">{t("ext.across.versions")}</th>
              <th scope="col">{t("ext.across.updates")}</th>
              <th scope="col">{t("ext.col.finding")}</th>
            </tr>
          </thead>
          {items.map((e) => {
            const key = `${e.kind}:${e.slug}`;
            const opened = open.has(key);
            const sites = e.sites ?? [];
            const versions = [...new Set(sites.map((s) => s.version))];
            return (
              <tbody key={key}>
                <tr>
                  <td>
                    <button
                      type="button"
                      className={styles.toggle}
                      aria-expanded={opened}
                      aria-label={t("ext.across.sitesOf", { name: e.title || e.slug })}
                      onClick={() => toggle(key)}
                    >
                      <Icon name={opened ? "caretDown" : "caretRight"} size={12} />
                    </button>
                  </td>
                  <td>
                    <span className={styles.name}>
                      {e.onWordPressOrg && onOpen ? (
                        <ExternalLink href={wordPressOrgAddress(e)} onOpen={onOpen} tone="text" size="body">
                          {e.title || e.slug}
                        </ExternalLink>
                      ) : (
                        <span className={styles.strong}>{e.title || e.slug}</span>
                      )}
                      <span className={styles.kind}>{t(`ext.across.kind.${e.kind}`)}</span>
                    </span>
                  </td>
                  <td className={styles.number}>{t("ext.across.of", { count: sites.length, total: across.sites })}</td>
                  <td className={styles.number}>{versions.length === 1 ? versions[0] : t("ext.across.versionCount", { count: versions.length })}</td>
                  <td>
                    {e.updates > 0 ? (
                      <span className={styles.inline}>
                        <StatusDot status="warn" />
                        {t("ext.across.updateCount", { count: e.updates })}
                      </span>
                    ) : (
                      <span className={styles.muted}>{t("overview.upToDate")}</span>
                    )}
                  </td>
                  <td className={styles.finding}>
                    {e.worst && (
                      <span className={styles.inline}>
                        <StatusDot status={e.worst.level === "urgent" ? "danger" : "warn"} />
                        <span className={styles.ellipsis} title={findingText(e.worst, t, i18n.language).title}>
                          {findingText(e.worst, t, i18n.language).title}
                        </span>
                      </span>
                    )}
                  </td>
                </tr>
                {opened &&
                  sites.map((s) => (
                    <tr key={s.id} className={styles.site}>
                      <td />
                      <td>
                        <button type="button" className={styles.siteName} onClick={() => onSelect(s.id)}>
                          {s.name}
                        </button>
                      </td>
                      <td />
                      <td className={styles.number}>{s.version}</td>
                      <td>
                        {s.update === "available" ? (
                          <span className={styles.inline}>
                            <StatusDot status="warn" />
                            {t("ext.to", { version: s.updateVersion })}
                          </span>
                        ) : (
                          <span className={styles.muted}>{t("overview.upToDate")}</span>
                        )}
                      </td>
                      <td className={styles.finding}>
                        {s.worst && (
                          <span className={styles.inline}>
                            <StatusDot status={s.worst.level === "urgent" ? "danger" : "warn"} />
                            <span className={styles.ellipsis}>{findingText(s.worst, t, i18n.language).title}</span>
                          </span>
                        )}
                      </td>
                    </tr>
                  ))}
              </tbody>
            );
          })}
        </table>
      )}
    </div>
  );
}
