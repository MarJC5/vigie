import { useTranslation } from "react-i18next";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./NavRail.module.css";

export type Section = "sites" | "activity" | "settings" | "help";

const sections: Array<{ id: Section; icon: IconName }> = [
  { id: "sites", icon: "sites" },
  { id: "activity", icon: "activity" },
  { id: "settings", icon: "settings" },
  { id: "help", icon: "help" },
];

export interface NavRailProps {
  current: Section;
  /** Opens the "add a host" wizard; the button is disabled while it is not available. */
  onAddHost?: () => void;
}

/** The narrow navigation rail; the native window controls sit above it. */
export function NavRail({ current, onAddHost }: NavRailProps) {
  const { t } = useTranslation();
  return (
    <nav aria-label={t("nav.navigation")} className={styles.rail}>
      {sections.map(({ id, icon }) => (
        <a
          key={id}
          href={`#${id}`}
          aria-label={t(`nav.${id}`)}
          aria-current={id === current ? "page" : undefined}
          className={styles.item}
        >
          <Icon name={icon} size={22} />
        </a>
      ))}
      <span className={styles.spacer} />
      <button
        type="button"
        aria-label={t("action.addHost")}
        className={styles.add}
        disabled={!onAddHost}
        onClick={onAddHost}
      >
        <Icon name="plus" size={20} />
      </button>
    </nav>
  );
}
