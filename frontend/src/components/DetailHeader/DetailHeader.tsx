import type { ReactNode } from "react";

import { Icon, type IconName } from "../Icon/Icon";
import styles from "./DetailHeader.module.css";

export interface DetailHeaderProps {
  title: string;
  icon?: IconName;
  /** A line under the title, for example the connection of a host. */
  subtitle?: string;
  /** On the right, for example the time of the last scan. */
  meta?: string;
  /** Actions after the title, for example a menu button. */
  children?: ReactNode;
  /** The main actions of the page, on the right of the title line. */
  actions?: ReactNode;
  /** Under the title, for example the quick links of a site. */
  links?: ReactNode;
}

/** The top of a detail pane: a very large name, quiet facts around it. */
export function DetailHeader({ title, icon, subtitle, meta, children, actions, links }: DetailHeaderProps) {
  return (
    <div className={styles.header}>
      <div className={styles.line}>
        {icon && (
          <span className={styles.icon}>
            <Icon name={icon} size={28} />
          </span>
        )}
        <h1 className={styles.title} title={title}>
          {title}
        </h1>
        {children}
        {meta && (
          <span className={styles.meta}>
            <Icon name="activity" size={14} />
            {meta}
          </span>
        )}
        {actions && <div className={styles.actions}>{actions}</div>}
      </div>
      {subtitle && (
        <p className={styles.subtitle} data-selectable>
          {subtitle}
        </p>
      )}
      {links}
    </div>
  );
}
