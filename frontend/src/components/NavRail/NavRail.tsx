import { useTranslation } from "react-i18next";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./NavRail.module.css";

export type Section = "sites" | "activity" | "addons" | "settings" | "help";

const sections: Array<{ id: Section; icon: IconName }> = [
  { id: "sites", icon: "vigie" },
  { id: "activity", icon: "activity" },
  { id: "addons", icon: "puzzle" },
  { id: "settings", icon: "settings" },
  { id: "help", icon: "help" },
];

export interface NavRailProps {
  current: Section;
  /** Opens the "add a host" wizard; the button is disabled while it is not available. */
  onAddHost?: () => void;
  /** Under a dialog: out of reach of the focus and the pointer. */
  inert?: boolean;
  /** Opens a section; without it, the rail only shows where the window is. */
  onNavigate?: (section: Section) => void;
}

// Every section can be reached; one left out is shown, not reachable yet.
const reachable: Section[] = ["sites", "activity", "addons", "settings", "help"];

/** The narrow navigation rail; the native window controls sit above it. */
export function NavRail({ current, onAddHost, inert, onNavigate }: NavRailProps) {
  const { t } = useTranslation();
  return (
    <nav aria-label={t("nav.navigation")} className={styles.rail} inert={inert}>
      {sections.map(({ id, icon }) => (
        <a
          key={id}
          href={`#${id}`}
          aria-label={t(`nav.${id}`)}
          aria-current={id === current ? "page" : undefined}
          aria-disabled={reachable.includes(id) ? undefined : true}
          className={styles.item}
          onClick={(event) => {
            event.preventDefault();
            if (onNavigate && reachable.includes(id)) onNavigate(id);
          }}
        >
          <Icon name={icon} size="nav" />
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
        <Icon name="plus" size="nav" />
      </button>
    </nav>
  );
}
