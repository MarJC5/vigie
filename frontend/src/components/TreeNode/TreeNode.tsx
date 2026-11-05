import { type CSSProperties, type DragEvent, type KeyboardEvent, type MouseEvent, useRef } from "react";
import { useTranslation } from "react-i18next";

import { Icon } from "../Icon/Icon";
import { InlineField } from "../InlineField/InlineField";
import { type Status, StatusDot } from "../StatusDot/StatusDot";
import styles from "./TreeNode.module.css";

export type SiteColor = "red" | "green" | "blue" | "yellow" | "cyan" | "magenta" | "orange";

export interface TreeNodeProps {
  /** A tag's row opens the page of its sites (ADR-0025); a subsite's, its page under its network
   * (ADR-0029). */
  kind: "folder" | "host" | "site" | "tag" | "subsite";
  name: string;
  /** 0 at the root. */
  depth: number;
  /** Folders and hosts; a network, which unfolds to its subsites. */
  expanded?: boolean;
  selected?: boolean;
  /** Sites only. */
  status?: Status;
  color?: SiteColor;
  /** A short note after the name, for example a count. */
  meta?: string;
  /** Hosts only: the updates waiting on its sites, in a badge when there are some. */
  updates?: number;
  /** A click or Enter; with ⌘ (or Ctrl) the item is added or taken out of the selection, with ⇧
   the visible items up to it are selected (ADR-0034). */
  onSelect: (gesture?: SelectGesture) => void;
  onToggle?: () => void;
  /** Reachable with Tab; by default only the selected item is (roving focus). */
  tabbable?: boolean;
  /** Its ID in the tree, on data-id. */
  itemId?: string;
  /** Enter on the selected item asks to rename it. */
  onEdit?: () => void;
  /** Shows a field instead of the name: Enter or leaving keeps it, Escape cancels. */
  editing?: boolean;
  /** The field's accessible name, for example "Name of Personal". */
  editLabel?: string;
  onRename?: (name: string) => void;
  onCancelEdit?: () => void;
  /** Folders and hosts can be dragged; sites stay under their host. */
  draggable?: boolean;
  /** Being dragged: dimmed. */
  dragging?: boolean;
  /** A folder the dragged item would go into: outlined. */
  dropTarget?: boolean;
  onDragStart?: (event: DragEvent) => void;
  onDragEnd?: (event: DragEvent) => void;
  onDragOver?: (event: DragEvent) => void;
  onDrop?: (event: DragEvent) => void;
  onContextMenu?: (event: MouseEvent) => void;
}

/** How an item was selected: alone, added or taken out (toggle), or with those before it (range). */
export interface SelectGesture {
  toggle?: boolean;
  range?: boolean;
}

export function TreeNode({
  kind, name, depth, expanded, selected = false, status, color, meta, updates = 0, onSelect, onToggle, tabbable, itemId, onEdit,
  editing = false, editLabel, onRename, onCancelEdit, draggable = false, dragging = false, dropTarget = false,
  onDragStart, onDragEnd, onDragOver, onDrop, onContextMenu,
}: TreeNodeProps) {
  const { t } = useTranslation();
  const branch = kind === "folder" || kind === "host" || (kind === "site" && expanded !== undefined);
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && selected && onEdit) {
      event.preventDefault();
      onEdit();
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      // Ctrl-Space, and ⌘-Space, toggle as ⌘-click does (the APG's model with modifier keys).
      onSelect({ toggle: event.key === " " && (event.ctrlKey || event.metaKey) });
    } else if (branch && ((event.key === "ArrowLeft" && expanded) || (event.key === "ArrowRight" && !expanded))) {
      event.preventDefault();
      onToggle?.();
    }
  };
  const style = {
    // A site's mark sits in the column of its host's tile (past the host's caret and gap), a
    // network's too, its caret before; a subsite's name under its network's (past the mark's
    // column); a tag's icon in the column of a host's caret.
    "--vg-node-indent": `${depth * 14 + (kind === "site" ? (branch ? -14 : 6) : kind === "subsite" ? 21 : 0)}px`,
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
      data-kind={kind}
      data-colored={color ? true : undefined}
      data-attention={status === "warn" || status === "danger" || undefined}
      data-id={itemId}
      data-dragging={dragging || undefined}
      data-drop-target={dropTarget || undefined}
      draggable={draggable && !editing}
      onClick={(event) => onSelect({ toggle: event.metaKey || event.ctrlKey, range: event.shiftKey })}
      onKeyDown={editing ? undefined : onKeyDown}
      onDragStart={onDragStart}
      onDragEnd={onDragEnd}
      onDragOver={onDragOver}
      onDrop={onDrop}
      onContextMenu={onContextMenu}
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
          <Icon name={expanded ? "caretDown" : "caretRight"} size="xs" />
        </span>
      )}
      {kind === "host" && (
        <span className={styles.tile}>
          <Icon name="host" size="sm" />
        </span>
      )}
      {kind === "tag" && (
        <span className={styles.tagIcon}>
          <Icon name="tag" size="sm" />
        </span>
      )}
      {status && (
        <span className={styles.mark}>
          <StatusDot status={status} />
        </span>
      )}
      {editing ? (
        <RenameField label={editLabel} name={name} onRename={onRename} onCancel={onCancelEdit} />
      ) : (
        <span className={styles.name}>{name}</span>
      )}
      {meta && <span className={styles.meta}>{meta}</span>}
      {updates > 0 && (
        <span className={styles.badge} role="img" aria-label={t("manager.pending", { count: updates })}>
          {updates}
        </span>
      )}
      {kind !== "host" && color && <span className={styles.tag} aria-hidden="true" />}
    </div>
  );
}

// The name being edited. It ends once: Enter, leaving or Escape; a blur after the end (the field
// going away) changes nothing, and an unchanged name is a cancel.
function RenameField({ label, name, onRename, onCancel }: { label?: string; name: string; onRename?: (name: string) => void; onCancel?: () => void }) {
  const done = useRef(false);
  const end = (fn: () => void) => {
    if (done.current) return;
    done.current = true;
    fn();
  };
  // An unchanged name is no rename.
  const keep = (value: string) => end(() => (value.trim() === name ? onCancel?.() : onRename?.(value)));
  return (
    <InlineField
      label={label ?? name}
      defaultValue={name}
      autoFocus
      onFocus={(event) => event.target.select()}
      onClick={(event) => event.stopPropagation()}
      // The field keeps its keys: the tree does not move or select while typing.
      onKeyDown={(event) => event.stopPropagation()}
      onCommit={keep}
      onCancel={() => end(() => onCancel?.())}
      onBlur={(event) => keep(event.target.value)}
    />
  );
}
