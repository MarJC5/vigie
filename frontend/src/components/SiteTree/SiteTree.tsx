import { type DragEvent, type KeyboardEvent, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import type { Status } from "../StatusDot/StatusDot";
import { type SiteColor, TreeNode } from "../TreeNode/TreeNode";
import styles from "./SiteTree.module.css";

export interface SiteTreeProps {
  label: string;
  /** Depth first, parents before their children, as SiteManager.Tree returns them. */
  nodes: Node[];
  selectedId?: string;
  onSelect: (id: string) => void;
  /** Shows what matches, and the folders and hosts that lead to it; case and accents ignored. */
  filter?: string;
  /** The item shown as a name field. */
  editingId?: string;
  /** Enter on the selected item. */
  onEdit?: (id: string) => void;
  onRename?: (id: string, name: string) => void;
  onCancelEdit?: () => void;
  /** Command-Backspace or Delete on an item. */
  onDelete?: (id: string) => void;
  /** Right click, the Menu key or Shift-F10; the point is in the window's coordinates. */
  onMenu?: (id: string, point: { x: number; y: number }) => void;
  /** A folder or host dropped into a folder ("" for the root). */
  onMove?: (id: string, folderId: string) => void;
  /** Follows a drag: the target folder ("" the root), undefined when there is none; null at the end. */
  onDragChange?: (drag: Drag | null) => void;
}

export interface Drag {
  id: string;
  target: string | undefined;
}

/** The site manager tree: folders and hosts fold, arrow keys move between visible items. */
export function SiteTree({
  label, nodes, selectedId, onSelect, filter = "", editingId, onEdit, onRename, onCancelEdit, onDelete, onMenu, onMove,
  onDragChange,
}: SiteTreeProps) {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const ref = useRef<HTMLDivElement>(null);
  const byId = useMemo(() => new Map(nodes.map((n) => [n.id, n])), [nodes]);

  // The drag in a ref, read by the next drag event at once; in state, for the outlines.
  const dragging = useRef<Drag | null>(null);
  const [drag, setDrag] = useState<Drag | null>(null);
  const follow = (next: Drag | null) => {
    const current = dragging.current;
    if (current?.id === next?.id && current?.target === next?.target) return;
    dragging.current = next;
    setDrag(next);
    onDragChange?.(next);
  };

  // Where a drop on a row goes: into a folder; on a host or a site, into that row's folder.
  const targetOf = (n: Node): string => {
    if (n.kind === "folder") return n.id;
    if (n.kind === "host") return n.parentId;
    return byId.get(n.parentId)?.parentId ?? "";
  };
  // A folder or host can go elsewhere than where it is, a folder never into itself.
  const allowed = (id: string, target: string): boolean => {
    const moved = byId.get(id);
    if (!moved || moved.kind === "site" || moved.parentId === target) return false;
    for (let f: string | undefined = target; f; f = byId.get(f)?.parentId) if (f === moved.id) return false;
    return true;
  };
  const over = (event: DragEvent, target: string) => {
    const current = dragging.current;
    if (!current) return;
    event.stopPropagation();
    if (allowed(current.id, target)) {
      event.preventDefault();
      event.dataTransfer.dropEffect = "move";
      follow({ id: current.id, target });
    } else {
      follow({ id: current.id, target: undefined });
    }
  };
  const drop = (event: DragEvent, target: string) => {
    const current = dragging.current;
    event.preventDefault();
    event.stopPropagation();
    follow(null);
    if (current && allowed(current.id, target)) onMove?.(current.id, target);
  };

  const visible = useMemo(() => {
    const byId = new Map(nodes.map((n) => [n.id, n]));
    const depthOf = (n: Node): number => (n.parentId ? depthOf(byId.get(n.parentId)!) + 1 : 0);
    const query = fold(filter.trim());
    if (query) {
      // Matches and their parents, all unfolded.
      const shown = new Set<string>();
      for (const n of nodes) {
        if (!fold(n.name).includes(query)) continue;
        for (let p: Node | undefined = n; p; p = byId.get(p.parentId)) shown.add(p.id);
      }
      return nodes.filter((n) => shown.has(n.id)).map((n) => ({ node: n, depth: depthOf(n) }));
    }
    const hidden = (n: Node): boolean => {
      for (let p = byId.get(n.parentId); p; p = byId.get(p.parentId)) if (collapsed.has(p.id)) return true;
      return false;
    };
    return nodes.filter((n) => !hidden(n)).map((n) => ({ node: n, depth: depthOf(n) }));
  }, [nodes, collapsed, filter]);

  const toggle = (id: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const onKeyDown = (event: KeyboardEvent) => {
    const row = document.activeElement as HTMLElement | null;
    const id = row?.dataset.id;
    if (id && ((event.metaKey && event.key === "Backspace") || event.key === "Delete")) {
      event.preventDefault();
      onDelete?.(id);
      return;
    }
    if (id && (event.key === "ContextMenu" || (event.shiftKey && event.key === "F10"))) {
      event.preventDefault();
      const rect = row!.getBoundingClientRect();
      onMenu?.(id, { x: rect.left + 24, y: rect.bottom });
      return;
    }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const items = [...(ref.current?.querySelectorAll<HTMLElement>("[role=treeitem]") ?? [])];
    const index = items.indexOf(document.activeElement as HTMLElement);
    const next = items[index + (event.key === "ArrowDown" ? 1 : -1)];
    if (next) {
      event.preventDefault();
      next.focus();
    }
  };

  const hasSelection = visible.some(({ node }) => node.id === selectedId);

  return (
    <div
      ref={ref}
      role="tree"
      aria-label={label}
      className={styles.tree}
      data-drop-target={drag?.target === "" || undefined}
      onKeyDown={onKeyDown}
      onDragOver={(event) => over(event, "")}
      onDrop={(event) => drop(event, "")}
    >
      {visible.map(({ node, depth }, index) => (
        <TreeNode
          key={node.id}
          kind={node.kind as "folder" | "host" | "site"}
          name={node.name}
          depth={depth}
          expanded={node.kind === "site" ? undefined : filter.trim() !== "" || !collapsed.has(node.id)}
          selected={node.id === selectedId}
          tabbable={hasSelection ? undefined : index === 0}
          status={node.kind === "site" ? siteStatus(node.status) : undefined}
          color={node.color === "none" ? undefined : (node.color as SiteColor)}
          meta={node.multisite && node.subsites > 0 ? t("manager.subsites", { count: node.subsites }) : undefined}
          onSelect={() => onSelect(node.id)}
          onToggle={() => toggle(node.id)}
          itemId={node.id}
          onEdit={onEdit && (() => onEdit(node.id))}
          editing={node.id === editingId}
          editLabel={t("manager.nameOf", { name: node.name })}
          onRename={(name) => onRename?.(node.id, name)}
          onCancelEdit={onCancelEdit}
          draggable={Boolean(onMove) && node.kind !== "site"}
          dragging={drag?.id === node.id}
          dropTarget={node.kind === "folder" && drag?.target === node.id}
          onDragStart={(event) => {
            event.dataTransfer.effectAllowed = "move";
            event.dataTransfer.setData("text/plain", node.name);
            follow({ id: node.id, target: undefined });
          }}
          onDragEnd={() => follow(null)}
          onDragOver={(event) => over(event, targetOf(node))}
          onDrop={(event) => drop(event, targetOf(node))}
          onContextMenu={
            onMenu &&
            ((event) => {
              event.preventDefault();
              onMenu(node.id, { x: event.clientX, y: event.clientY });
            })
          }
        />
      ))}
    </div>
  );
}

// The latest scan's status; a site never scanned shows the unknown ring.
function siteStatus(status: string): Status {
  return status === "ok" || status === "warn" || status === "danger" ? status : "unknown";
}

// Lower case without accents, so "écoles" matches "ECOLES".
function fold(text: string): string {
  return text.normalize("NFD").replace(/\p{Diacritic}/gu, "").toLowerCase();
}
