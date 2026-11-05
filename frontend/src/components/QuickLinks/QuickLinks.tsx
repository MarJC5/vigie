import type { IconName } from "../Icon/Icon";
import { LinkButton } from "../LinkButton/LinkButton";
import styles from "./QuickLinks.module.css";

export interface QuickLink {
  label: string;
  icon: IconName;
  onClick: () => void;
}

export interface QuickLinksProps {
  label: string;
  links: QuickLink[];
}

/** A quiet row of actions under a title, an icon and a label each, thin rules between them. */
export function QuickLinks({ label, links }: QuickLinksProps) {
  if (links.length === 0) return null;
  return (
    <ul aria-label={label} className={styles.links}>
      {/* By position: a link may change its label and icon ("Copied") and keeps its focus. */}
      {links.map((link, index) => (
        <li key={index} className={styles.item}>
          <LinkButton tone="quiet" icon={link.icon} onClick={link.onClick}>
            {link.label}
          </LinkButton>
        </li>
      ))}
    </ul>
  );
}
