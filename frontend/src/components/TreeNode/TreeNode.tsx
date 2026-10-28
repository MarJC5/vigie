import type { CSSProperties, KeyboardEvent } from "react";

import { Icon } from "../Icon/Icon";
import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./TreeNode.module.css";

export type SiteColor = "red" | "green" | "blue" | "yellow" | "cyan" | "magenta" | "orange";

export interface TreeNodeProps {
  kind: "folder" | "host" | "site";
  name: string;
  /** 0 at the root. */
  depth: number;
  /** Folders and hosts only. */
  expanded?: boolean;
  selected?: boolean;
  /** Sites only. */
  status?: Status;
  color?: SiteColor;
  /** A short note after the name, for example a count. */
  meta?: string;
  onSelect: () => void;
  onToggle?: () => void;
  /** Reachable with Tab; by default only the selected item is (roving focus). */
  tabbable?: boolean;
}

export function TreeNode({ kind, name, depth, expanded, selected = false, status, color, meta, onSelect, onToggle, tabbable }: TreeNodeProps) {
  const branch = kind !== "site";
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      onSelect();
    } else if (branch && ((event.key === "ArrowLeft" && expanded) || (event.key === "ArrowRight" && !expanded))) {
      event.preventDefault();
      onToggle?.();
    }
  };
  const style = {
    "--vg-node-indent": `${depth * 18 + (branch ? 0 : 18)}px`,
    ...(color ? { "--vg-node-color": `var(--vg-site-${color})` } : {}),
  } as CSSProperties;

  return (
    <div
      role="treeitem"
      aria-level={depth + 1}
      aria-selected={selected}
      aria-expanded={branch ? Boolean(expanded) : undefined}
      tabIndex={(tabbable ?? selected) ? 0 : -1}
      className={styles.node}
      style={style}
      onClick={onSelect}
      onKeyDown={onKeyDown}
    >
      {branch && (
        <span
          className={styles.caret}
          data-caret
          onClick={(event) => {
            event.stopPropagation();
            onToggle?.();
          }}
        >
          <Icon name={expanded ? "caretDown" : "caretRight"} size={12} />
        </span>
      )}
      {branch && (
        <span className={color ? styles.colored : styles.icon}>
          <Icon name={kind === "host" ? "host" : expanded ? "folderOpen" : "folder"} size={16} />
        </span>
      )}
      {status && <StatusDot status={status} />}
      <span className={styles.name}>{name}</span>
      {meta && <span className={styles.meta}>{meta}</span>}
      {!branch && color && <span className={styles.tag} aria-hidden="true" />}
    </div>
  );
}
