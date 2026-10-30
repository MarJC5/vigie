import { useId, useState } from "react";
import { useTranslation } from "react-i18next";

import type { AddonsView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { addonsDocs } from "../../lib/addons";
import { AddonCard } from "../AddonCard/AddonCard";
import { AddonTrustDialog } from "../AddonTrustDialog/AddonTrustDialog";
import { Button } from "../Button/Button";
import { Icon } from "../Icon/Icon";
import { TabBar } from "../TabBar/TabBar";
import styles from "./AddonsPage.module.css";

export type AddonsTab = "library" | "installed";

export interface AddonsPageProps {
  /** What Vigie found; null while it asks each add-on for its description. */
  view: AddonsView | null;
  tab: AddonsTab;
  onTab: (tab: AddonsTab) => void;
  onOpen: (name: string) => void;
  /** Opens an address in the browser. */
  onLink: (href: string) => void;
}

/** The add-ons as a grid of cards, in two tabs: the Library and the ones installed. */
export function AddonsPage({ view, tab, onTab, onOpen, onLink }: AddonsPageProps) {
  const { t } = useTranslation();
  const panel = useId();
  const [trust, setTrust] = useState(false);
  const addons = (tab === "library" ? view?.library : view?.installed) ?? [];
  return (
    <div className={styles.page}>
      <div className={styles.head}>
        <h1 className={styles.title}>{t("addons.title")}</h1>
        <span className={styles.actions}>
          <Button variant="icon" icon="lock" label={t("addons.trust")} title={t("addons.trust")} onClick={() => setTrust(true)} className={styles.lock} />
          <a
            href={addonsDocs}
            className={styles.write}
            onClick={(event) => {
              event.preventDefault();
              onLink(addonsDocs);
            }}
          >
            {t("addons.write")}
            <Icon name="external" size={13} />
          </a>
        </span>
      </div>
      <TabBar
        label={t("addons.title")}
        tabs={(["library", "installed"] as const).map((id) => ({ id, label: t(`addons.tabs.${id}`) }))}
        value={tab}
        onChange={(id) => onTab(id as AddonsTab)}
        panelId={panel}
      />
      <div role="tabpanel" id={panel} aria-labelledby={`${panel}-${tab}`} className={styles.panel}>
        {!view && <p className={styles.note}>{t("addons.loading")}</p>}
        {view && addons.length === 0 && tab === "installed" && (
          <div className={styles.empty}>
            <span className={styles.emptyIcon}>
              <Icon name="puzzle" size={30} />
            </span>
            <h2 className={styles.emptyTitle}>{t("addons.emptyTitle")}</h2>
            <p className={styles.note}>{t("addons.emptyText")}</p>
          </div>
        )}
        {addons.length > 0 && (
          <div className={styles.grid}>
            {addons.map((a) => (
              <AddonCard key={a.name} addon={a} tab={tab} onOpen={onOpen} onLink={onLink} />
            ))}
          </div>
        )}
      </div>
      {trust && <AddonTrustDialog onClose={() => setTrust(false)} />}
    </div>
  );
}
