import type { ReactNode } from "react";

import skin from "../../styles/link.module.css";
import { Icon } from "../Icon/Icon";
import styles from "./ExternalLink.module.css";

export interface ExternalLinkProps {
  href: string;
  /** Opens the address in the browser; without it, the link opens a new tab. */
  onOpen?: (href: string) => void;
  children: ReactNode;
  /** accent (default): a link among other text; text: a name that leads somewhere, in the text's color. */
  tone?: "accent" | "text";
  /** small (default) for actions and notes, body for a name in a table; inline for a link in running text. */
  size?: "small" | "body" | "inline";
}

/** A link that leaves the window: its text, then the external icon, which never shrinks and stays
 * at the end of the last line when the text wraps. The one way the window shows such a link; its
 * skin is LinkButton's (styles/link.module.css). */
export function ExternalLink({ href, onOpen, children, tone = "accent", size = "small" }: ExternalLinkProps) {
  return (
    <a
      href={href}
      target="_blank"
      rel="noreferrer"
      className={skin.link}
      data-tone={tone}
      data-size={size}
      onClick={(event) => {
        if (!onOpen) return;
        event.preventDefault();
        onOpen(href);
      }}
    >
      {glued(children)}
    </a>
  );
}

/** The text with the icon glued to its last word, so that the icon never goes to a line alone. */
function glued(children: ReactNode): ReactNode {
  const icon = (
    <span className={styles.icon} aria-hidden="true">
      <Icon name="external" size={13} />
    </span>
  );
  if (typeof children !== "string") {
    return (
      <>
        {children}
        {icon}
      </>
    );
  }
  const space = children.trimEnd().lastIndexOf(" ");
  return (
    <>
      {children.slice(0, space + 1)}
      <span className={styles.last}>
        {children.slice(space + 1)}
        {icon}
      </span>
    </>
  );
}
