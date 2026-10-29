import { type CSSProperties, type DragEvent, type KeyboardEvent, type MouseEvent, useRef } from "react";

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

export function TreeNode({
  kind, name, depth, expanded, selected = false, status, color, meta, onSelect, onToggle, tabbable, itemId, onEdit,
  editing = false, editLabel, onRename, onCancelEdit, draggable = false, dragging = false, dropTarget = false,
  onDragStart, onDragEnd, onDragOver, onDrop, onContextMenu,
}: TreeNodeProps) {
  const branch = kind !== "site";
  const onKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && selected && onEdit) {
      event.preventDefault();
      onEdit();
    } else if (event.key === "Enter" || event.key === " ") {
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
      data-id={itemId}
      data-dragging={dragging || undefined}
      data-drop-target={dropTarget || undefined}
      draggable={draggable && !editing}
      onClick={onSelect}
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
          <Icon name={expanded ? "caretDown" : "caretRight"} size={12} />
        </span>
      )}
      {branch && (
        <span className={color ? styles.colored : styles.icon}>
          <Icon name={kind === "host" ? "host" : expanded ? "folderOpen" : "folder"} size={16} />
        </span>
      )}
      {status && <StatusDot status={status} />}
      {editing ? (
        <RenameField label={editLabel} name={name} onRename={onRename} onCancel={onCancelEdit} />
      ) : (
        <span className={styles.name}>{name}</span>
      )}
      {meta && <span className={styles.meta}>{meta}</span>}
      {!branch && color && <span className={styles.tag} aria-hidden="true" />}
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
    <input
      type="text"
      aria-label={label}
      defaultValue={name}
      className={styles.field}
      autoFocus
      spellCheck={false}
      onFocus={(event) => event.target.select()}
      onClick={(event) => event.stopPropagation()}
      onKeyDown={(event) => {
        // The field keeps its keys: the tree does not move or select while typing.
        event.stopPropagation();
        const value = event.currentTarget.value;
        if (event.key === "Enter") keep(value);
        if (event.key === "Escape") end(() => onCancel?.());
      }}
      onBlur={(event) => keep(event.target.value)}
    />
  );
}
