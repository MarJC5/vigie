import type { ButtonHTMLAttributes, ReactNode } from "react";

import skin from "../../styles/link.module.css";
import { Icon, type IconName } from "../Icon/Icon";
import styles from "./LinkButton.module.css";

export interface LinkButtonProps extends Omit<ButtonHTMLAttributes<HTMLButtonElement>, "children" | "type" | "style"> {
  /** accent (default): an action, a finding, another page; text: the name of a row that opens it;
   * quiet: the shortcuts of a site. */
  tone?: "accent" | "text" | "quiet";
  /** small (default) for actions and notes, body for a name in a table, inline in running text. */
  size?: "small" | "body" | "inline";
  /** A leading icon. */
  icon?: IconName;
  /** Accessible name, when the text alone would not say enough. */
  label?: string;
  children: ReactNode;
}

/** A button that reads as a link: ExternalLink's counterpart for what stays in the window, with
 * its skin (ADR-0004). */
export function LinkButton({ tone = "accent", size = "small", icon, label, children, className, ...rest }: LinkButtonProps) {
  return (
    <button
      type="button"
      {...rest}
      aria-label={label}
      data-tone={tone}
      data-size={size}
      className={[skin.link, styles.button, className].filter(Boolean).join(" ")}
    >
      {icon && <Icon name={icon} size={14} />}
      {children}
    </button>
  );
}
