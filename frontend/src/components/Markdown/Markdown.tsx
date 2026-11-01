import { Children, isValidElement, type ReactNode, useMemo } from "react";
import ReactMarkdown, { type Components } from "react-markdown";
import remarkGfm from "remark-gfm";

import { ExternalLink } from "../ExternalLink/ExternalLink";
import styles from "./Markdown.module.css";

export interface MarkdownProps {
  /** The text, in GitHub's Markdown (tables, autolinks). Raw HTML in it is shown as text. */
  text: string;
  /** Called with the href of a link clicked: the window never follows a link itself. */
  onLink: (href: string) => void;
  /** The level of the text's first heading on the page, 2 under the page's own title. */
  level?: 2 | 3;
  /** Whether a link leaves the window, marked with an icon; by default, a web address. */
  external?: (href: string) => boolean;
}

/** The id of a heading: its text in lowercase, without accents, its words joined by "-". */
export function headingId(text: string): string {
  const words = text
    .normalize("NFD")
    .replace(/\p{Diacritic}/gu, "")
    .toLowerCase()
    .match(/[\p{L}\p{N}]+/gu);
  return "doc-" + (words ?? []).join("-");
}

/** The second-level headings of a text (## in Markdown), for a table of contents. */
export function outline(text: string): Array<{ id: string; text: string }> {
  const headings: Array<{ id: string; text: string }> = [];
  let fenced = false;
  for (const line of text.split("\n")) {
    if (line.startsWith("```")) fenced = !fenced;
    if (fenced || !line.startsWith("## ")) continue;
    const plain = line
      .slice(3)
      .replace(/\[([^\]]*)\]\([^)]*\)/g, "$1")
      .replace(/[`*_]/g, "")
      .trim();
    headings.push({ id: headingId(plain), text: plain });
  }
  return headings;
}

/** The text of rendered children, for a heading's id. */
function plainText(children: ReactNode): string {
  return Children.toArray(children)
    .map((child) => {
      if (typeof child === "string" || typeof child === "number") return String(child);
      if (isValidElement<{ children?: ReactNode }>(child)) return plainText(child.props.children);
      return "";
    })
    .join("");
}

const web = (href: string) => /^https?:\/\//.test(href);

/**
 * A doc of Vigie, rendered as components: no HTML of the text reaches the window (ADR-0020). Its
 * headings sit under the page's title, each with an id; links go to onLink; the text can be
 * selected and copied, commands included.
 */
export function Markdown({ text, onLink, level = 2, external = web }: MarkdownProps) {
  // Made once per level and onLink: a new component on each render would redraw the doc, and lose
  // the reader's selection.
  const components = useMemo((): Components => {
    const heading = (depth: number) => {
      const Tag = `h${Math.min(depth + level - 1, 6)}` as "h2";
      return function Heading({ children }: { children?: ReactNode }) {
        return (
          <Tag id={headingId(plainText(children))} className={styles[`h${depth}`]}>
            {children}
          </Tag>
        );
      };
    };
    return {
      h1: heading(1),
      h2: heading(2),
      h3: heading(3),
      h4: heading(4),
      a: ({ href = "", children }) =>
        external(href) ? (
          <ExternalLink href={href} onOpen={onLink} size="inline">
            {children}
          </ExternalLink>
        ) : (
          <a
            href={href}
            className={styles.link}
            onClick={(event) => {
              event.preventDefault();
              onLink(href);
            }}
          >
            {children}
          </a>
        ),
      table: ({ children }) => (
        <div className={styles.table}>
          <table>{children}</table>
        </div>
      ),
      // No image of a doc is loaded: the window reaches nothing outside itself.
      img: ({ alt }) => <span>{alt}</span>,
    };
  }, [level, onLink, external]);
  return (
    <div className={styles.doc} data-selectable>
      <ReactMarkdown remarkPlugins={[remarkGfm]} components={components}>
        {text}
      </ReactMarkdown>
    </div>
  );
}
