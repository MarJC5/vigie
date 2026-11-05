import { useTranslation } from "react-i18next";

import type { SiteView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { since } from "../../lib/time";
import { DetailHeader } from "../DetailHeader/DetailHeader";
import { ExtensionList } from "../ExtensionList/ExtensionList";
import { InfoAside, InfoList, InfoRow } from "../InfoList/InfoList";
import { LinkButton } from "../LinkButton/LinkButton";
import { Notice } from "../Notice/Notice";
import { type QuickLink, QuickLinks } from "../QuickLinks/QuickLinks";
import { FindingRows } from "../SecurityPanel/SecurityPanel";
import styles from "./SubsitePage.module.css";

export interface SubsitePageProps {
  /** The network, with its latest scan. */
  site: SiteView;
  blogId: number;
  /** Opens the network's page. */
  onNetwork: () => void;
  /** Opens an address in the browser: the subsite, its dashboard. */
  onOpen?: (href: string) => void;
  /** "Now", for the time since the last scan; the clock by default. */
  now?: Date;
}

/** A subsite of a network (ADR-0029), read-only: what the network's latest scan read of it. Nothing
 * to scan, update or edit on its own; its network does. */
export function SubsitePage({ site, blogId, onNetwork, onOpen, now }: SubsitePageProps) {
  const { t, i18n } = useTranslation();
  const sub = (site.subsites ?? []).find((s) => s.blogId === blogId);
  if (!sub) return null;
  const scan = site.scan;
  const title = sub.label === "/" ? site.name : sub.label.startsWith("/") ? `${site.name}${sub.label}` : sub.label;
  const links: QuickLink[] = onOpen
    ? [
        { label: t("site.openSite"), icon: "globe", onClick: () => onOpen(sub.url) },
        { label: t("site.wpAdmin"), icon: "external", onClick: () => onOpen(sub.adminUrl) },
      ]
    : [];
  const extensions = scan?.extensions ?? [];
  // Its own plugins, then those active network-wide, which run on it too.
  const here = extensions.filter((e) => e.kind === "plugin" && (sub.plugins ?? []).includes(e.slug));
  const network = extensions.filter((e) => e.kind === "plugin" && e.status === "active-network");
  const admins = (sub.admins ?? [])
    .map((login) => (scan?.findings ?? []).find((f) => f.kind === "administrator" && f.subject === `user:${login}`))
    .filter((f) => f !== undefined);
  const theme = [sub.themeName || sub.theme, sub.themeVersion].filter(Boolean).join(" ");

  return (
    <div className={styles.page}>
      <DetailHeader
        title={title}
        meta={scan ? t("site.lastScan", { when: since(scan.finished, i18n.language, now) }) : undefined}
        links={
          <>
            <p className={styles.of}>
              {t("subsite.of")}
              <LinkButton onClick={onNetwork}>
                {site.name}
              </LinkButton>
            </p>
            <QuickLinks label={t("site.links")} links={links} />
          </>
        }
      />
      {!sub.read && (
        <Notice
          tone="neutral"
          icon="archive"
          role="note"
          title={t(`subsite.stateLabel.${sub.state}`)}
          text={sub.state === "archived" || sub.state === "spam" || sub.state === "deleted" ? t("subsite.notRead", { state: t(`subsite.state.${sub.state}`) }) : t("subsite.notScanned")}
        />
      )}
      <InfoList>
        <InfoRow label={t("subsite.address")}>
          <span data-selectable>{sub.url}</span>
        </InfoRow>
        <InfoRow label={t("subsite.stateRow")}>{t(`subsite.stateLabel.${sub.state}`)}</InfoRow>
        {sub.read && (
          <>
            <InfoRow label={t("subsite.theme")}>{theme}</InfoRow>
            <InfoRow label={t("subsite.plugins")}>
              {t("subsite.pluginsHere", { count: here.length })}
              {network.length > 0 && <InfoAside>{t("subsite.pluginsNetwork", { count: network.length })}</InfoAside>}
            </InfoRow>
            {sub.adminEmail && (
              <InfoRow label={t("overview.adminEmail")}>
                <span data-selectable>{sub.adminEmail}</span>
              </InfoRow>
            )}
          </>
        )}
      </InfoList>
      {sub.read && (
        <>
          <section className={styles.section}>
            <h2 className={styles.title}>{t("subsite.plugins")}</h2>
            {here.length + network.length > 0 ? (
              <ExtensionList extensions={[...here, ...network]} subsites={site.subsites ?? []} onOpen={onOpen} compact />
            ) : (
              <p className={styles.empty}>{t("subsite.noPlugins")}</p>
            )}
          </section>
          <section className={styles.section}>
            <h2 className={styles.title}>{t("subsite.admins")}</h2>
            {admins.length > 0 ? (
              <FindingRows findings={admins} label={t("subsite.admins")} subsites={site.subsites ?? []} />
            ) : (
              <p className={styles.empty}>{t("subsite.noAdmins")}</p>
            )}
          </section>
        </>
      )}
    </div>
  );
}
