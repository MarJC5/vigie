import { useTranslation } from "react-i18next";

import type { AddonView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { textIn } from "../../lib/addons";
import { AddonImage } from "../AddonImage/AddonImage";
import { Button } from "../Button/Button";
import { Icon } from "../Icon/Icon";
import { ExternalLink } from "../ExternalLink/ExternalLink";
import styles from "./AddonCard.module.css";

export interface AddonCardProps {
  addon: AddonView;
  /** On the Installed tab, its page opens; in the Library, it says whether it is installed. */
  tab: "library" | "installed";
  onOpen: (name: string) => void;
  /** Opens where to get an add-on not installed. */
  onLink: (href: string) => void;
}

/** An add-on as a card of the grid: its image, title, author, a sentence, its kind and state. */
export function AddonCard({ addon, tab, onOpen, onLink }: AddonCardProps) {
  const { t, i18n } = useTranslation();
  const title = textIn(addon.title, i18n.language);
  return (
    <article className={styles.card} aria-label={title}>
      <AddonImage image={addon.image} />
      <div className={styles.body}>
        <h2 className={styles.title}>{title}</h2>
        <span className={styles.by}>{t("addons.by", { author: addon.author })}</span>
        <p className={styles.summary}>{textIn(addon.summary, i18n.language)}</p>
        <div className={styles.foot}>
          <span className={styles.kind}>{t(`addons.kind.${addon.kind}`)}</span>
          {tab === "installed" ? (
            <Button onClick={() => onOpen(addon.name)} label={t("addons.openNamed", { title })}>
              {t("addons.open")}
            </Button>
          ) : addon.installed ? (
            <span className={styles.note}>
              <Icon name="ok" size="sm" />
              {t("addons.installedNote")}
            </span>
          ) : (
            <ExternalLink href={addon.install} onOpen={onLink}>
              {t("addons.howTo")}
            </ExternalLink>
          )}
        </div>
      </div>
    </article>
  );
}
