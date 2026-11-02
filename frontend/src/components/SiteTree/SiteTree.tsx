import { type DragEvent, type KeyboardEvent, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node, TagView } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { openKey } from "../../lib/subsites";
import { tagId, tagsGroupId } from "../../lib/tags";
import { fold } from "../../lib/text";
import { Icon } from "../Icon/Icon";
import type { Status } from "../StatusDot/StatusDot";
import { type SelectGesture, type SiteColor, TreeNode } from "../TreeNode/TreeNode";
import styles from "./SiteTree.module.css";

export interface SiteTreeProps {
  label: string;
  /** Depth first, parents before their children, as SiteManager.Tree returns them. */
  nodes: Node[];
  selectedId?: string;
  onSelect: (id: string) => void;
  /** Several items selected (ADR-0034); without it, selectedId alone. */
  selectedIds?: string[];
  /** The selection changed by ⌘-click, ⇧-click, ⇧-arrows, ⌘A or Escape; without it, one item at a
   time. */
  onSelectionChange?: (ids: string[]) => void;
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
  /** Folders and hosts dropped into a folder ("" for the root): the one dragged, or the ones of the
   selection it is in. */
  onMove?: (ids: string[], folderId: string) => void;
  /** Follows a drag: the target folder ("" the root), undefined when there is none; null at the end. */
  onDragChange?: (drag: Drag | null) => void;
  /** The tags in use, in a Tags group under the folders (ADR-0025); a tag's row has the ID tagId(tag). */
  tags?: TagView[];
  /** The folders and hosts folded, when kept above the tree (so they outlive it); else its own. */
  collapsed?: Set<string>;
  onCollapsedChange?: (collapsed: Set<string>) => void;
}

/** What a row was found by, when it is not its name. */
type Found = "tags" | "notes" | undefined;

/** The text of folders and hosts dragged together, with their count: hosts, folders or items. */
export function carriedKey(kinds: (string | undefined)[]): string {
  const set = new Set(kinds);
  if (set.size > 1) return "manager.dragItems";
  return set.has("host") ? "manager.dragHosts" : "manager.dragFolders";
}

export interface Drag {
  /** The row grabbed. */
  id: string;
  /** What moves: the row, or the folders and hosts of the selection it is in. */
  ids: string[];
  target: string | undefined;
}

/** The site manager tree: folders and hosts fold, arrow keys move between visible items. */
export function SiteTree({
  label, nodes, selectedId, onSelect, selectedIds, onSelectionChange, filter = "", editingId, onEdit, onRename, onCancelEdit,
  onDelete, onMenu, onMove, onDragChange, tags, collapsed: kept, onCollapsedChange,
}: SiteTreeProps) {
  const { t } = useTranslation();
  const [own, setOwn] = useState<Set<string>>(new Set());
  const collapsed = kept ?? own;
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
  // Folders and hosts can go elsewhere than where they all are, a folder never into itself.
  const allowed = (ids: string[], target: string): boolean => {
    const moved = ids.map((id) => byId.get(id));
    if (moved.length === 0 || moved.some((n) => !n || n.kind === "site") || moved.every((n) => n!.parentId === target)) return false;
    for (let f: string | undefined = target; f; f = byId.get(f)?.parentId) if (ids.includes(f)) return false;
    return true;
  };
  const over = (event: DragEvent, target: string) => {
    const current = dragging.current;
    if (!current) return;
    event.stopPropagation();
    if (allowed(current.ids, target)) {
      event.preventDefault();
      event.dataTransfer.dropEffect = "move";
      follow({ ...current, target });
    } else {
      follow({ ...current, target: undefined });
    }
  };
  const drop = (event: DragEvent, target: string) => {
    const current = dragging.current;
    event.preventDefault();
    event.stopPropagation();
    follow(null);
    if (current && allowed(current.ids, target)) onMove?.(current.ids, target);
  };

  // The selection (ADR-0034): several folders, hosts and sites; a tag or a subsite alone.
  const selection = selectedIds && selectedIds.length > 0 ? selectedIds : selectedId ? [selectedId] : [];
  const several = selection.length > 1;
  const selectable = (id: string) => {
    const kind = byId.get(id)?.kind;
    return kind === "folder" || kind === "host" || kind === "site";
  };
  // Where a range starts: the item last chosen alone or with ⌘.
  const [anchor, setAnchor] = useState<string>();
  // What a drag of a row moves: the folders and hosts of the selection it is in, else the row.
  const moving = (id: string): string[] =>
    (several && selection.includes(id) ? selection : [id]).filter((m) => {
      const kind = byId.get(m)?.kind;
      return kind === "folder" || kind === "host";
    });
  // The label by the pointer when the selection's folders and hosts move: how many, of which kind.
  const dragLabel = useRef<HTMLSpanElement>(null);
  const carried = several && selection[0] ? moving(selection[0]) : [];
  const carriedKinds = new Set(carried.map((id) => byId.get(id)?.kind));
  const carriedText = t(carriedKey(carried.map((id) => byId.get(id)?.kind)), { count: carried.length });

  const visible = useMemo(() => {
    const byId = new Map(nodes.map((n) => [n.id, n]));
    const depthOf = (n: Node): number => (n.parentId ? depthOf(byId.get(n.parentId)!) + 1 : 0);
    const query = fold(filter.trim());
    if (query) {
      // Matches and their parents, all unfolded.
      const shown = new Set<string>();
      const inTags = (n: Node) => (n.tags ?? []).some((tag) => fold(tag).includes(query));
      for (const n of nodes) {
        if (!fold(n.name).includes(query) && !fold(n.notes).includes(query) && !inTags(n)) continue;
        for (let p: Node | undefined = n; p; p = byId.get(p.parentId)) shown.add(p.id);
      }
      // Found by its tags or its notes only: the row says so.
      const foundBy = (n: Node): Found => {
        if (fold(n.name).includes(query)) return undefined;
        return inTags(n) ? "tags" : fold(n.notes).includes(query) ? "notes" : undefined;
      };
      return nodes.filter((n) => shown.has(n.id)).map((n) => ({ node: n, depth: depthOf(n), found: foundBy(n) }));
    }
    // The item being named shows, even in a folded folder.
    const unfolded = new Set<string>();
    for (let p = editingId ? byId.get(byId.get(editingId)?.parentId ?? "") : undefined; p; p = byId.get(p.parentId)) unfolded.add(p.id);
    // A network is folded unless unfolded (openKey), the other branches unfolded unless folded.
    const folded = (p: Node) => (p.kind === "site" ? !collapsed.has(openKey(p.id)) && !unfolded.has(p.id) : collapsed.has(p.id) && !unfolded.has(p.id));
    const hidden = (n: Node): boolean => {
      for (let p = byId.get(n.parentId); p; p = byId.get(p.parentId)) if (folded(p)) return true;
      return false;
    };
    return nodes.filter((n) => !hidden(n)).map((n) => ({ node: n, depth: depthOf(n), found: undefined as Found }));
  }, [nodes, collapsed, filter, editingId]);

  // The Tags group, folded like a folder; a search keeps the tags that hold it, unfolded.
  const tagRows = useMemo(() => {
    const query = fold(filter.trim());
    const list = (tags ?? []).filter((tag) => !query || fold(tag.name).includes(query));
    if (list.length === 0) return [];
    const open = query !== "" || !collapsed.has(tagsGroupId);
    return [{ group: true, tag: undefined }, ...(open ? list.map((tag) => ({ group: false, tag })) : [])];
  }, [tags, filter, collapsed]);

  const toggle = (id: string) => {
    const next = new Set(collapsed);
    if (!next.delete(id)) next.add(id);
    (onCollapsedChange ?? setOwn)(next);
  };

  // The visible folders, hosts and sites, in order: what a range or ⌘A selects.
  const rows = visible.map(({ node }) => node.id).filter(selectable);
  const range = (from: string, to: string) => {
    const a = rows.indexOf(from);
    const b = rows.indexOf(to);
    if (a < 0 || b < 0) return [to];
    return rows.slice(Math.min(a, b), Math.max(a, b) + 1);
  };
  // A click or Enter on an item: alone, or with ⌘ added to or taken out of the selection, or with
  // ⇧ the items from the anchor to it.
  const choose = (id: string, gesture?: SelectGesture) => {
    if (!onSelectionChange || !selectable(id) || !(gesture?.toggle || gesture?.range)) {
      setAnchor(id);
      onSelect(id);
      return;
    }
    const base = selection.filter(selectable);
    const start = anchor ?? selection.at(-1);
    if (gesture.range && start && selectable(start)) {
      onSelectionChange(range(start, id));
      return;
    }
    setAnchor(id);
    onSelectionChange(base.includes(id) ? base.filter((s) => s !== id) : [...base, id]);
  };
  // The item a Tab reaches: the anchor of several, else the one selected.
  const focusTarget = several ? (anchor && selection.includes(anchor) ? anchor : selection.at(-1)) : undefined;

  const onKeyDown = (event: KeyboardEvent) => {
    const row = document.activeElement as HTMLElement | null;
    // The Tags group itself has no actions.
    // The Tags group, and a subsite (read-only, ADR-0029), have none either.
    const id = row?.dataset.id === tagsGroupId || row?.dataset.id?.startsWith("subsite:") ? undefined : row?.dataset.id;
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
    // A name being typed keeps its own ⌘A and Escape.
    const typing = (event.target as HTMLElement).tagName === "INPUT";
    if (onSelectionChange && !typing && (event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "a") {
      event.preventDefault();
      onSelectionChange(rows);
      return;
    }
    if (onSelectionChange && !typing && event.key === "Escape" && several) {
      event.preventDefault();
      const focused = row?.dataset.id;
      const kept = focused && selectable(focused) ? focused : selection.at(-1);
      if (kept) onSelectionChange([kept]);
      return;
    }
    if (event.key !== "ArrowDown" && event.key !== "ArrowUp") return;
    const items = [...(ref.current?.querySelectorAll<HTMLElement>("[role=treeitem]") ?? [])];
    const index = items.indexOf(document.activeElement as HTMLElement);
    const next = items[index + (event.key === "ArrowDown" ? 1 : -1)];
    if (next) {
      event.preventDefault();
      next.focus();
      // ⇧ and an arrow: the selection runs from the anchor to the item reached.
      const reached = next.dataset.id;
      if (event.shiftKey && onSelectionChange && reached && selectable(reached)) {
        const start = anchor ?? (selection.includes(row?.dataset.id ?? "") ? row?.dataset.id : selection.at(-1));
        const from = start && selectable(start) ? start : row?.dataset.id;
        if (from && selectable(from)) {
          setAnchor(from);
          onSelectionChange(range(from, reached));
        }
      }
    }
  };

  const hasSelection = visible.some(({ node }) => selection.includes(node.id)) || tagRows.some((r) => r.tag && selection.includes(tagId(r.tag.name)));

  return (
    <div
      ref={ref}
      role="tree"
      aria-label={label}
      aria-multiselectable={onSelectionChange ? true : undefined}
      className={styles.tree}
      data-several={several || undefined}
      data-drop-target={drag?.target === "" || undefined}
      onKeyDown={onKeyDown}
      onDragOver={(event) => over(event, "")}
      onDrop={(event) => drop(event, "")}
    >
      {visible.map(({ node, depth, found }, index) => {
        // A network unfolds to its subsites (ADR-0029), which are read-only.
        const network = node.kind === "site" && (node.subsiteList ?? []).length > 0;
        const subsite = node.kind === "subsite";
        return (
          <TreeNode
            key={node.id}
            kind={node.kind as "folder" | "host" | "site" | "subsite"}
            name={node.name}
            depth={depth}
            expanded={
              network ? filter.trim() !== "" || collapsed.has(openKey(node.id)) : node.kind === "site" || subsite ? undefined : filter.trim() !== "" || !collapsed.has(node.id)
            }
            selected={selection.includes(node.id)}
            tabbable={several ? node.id === focusTarget : hasSelection ? undefined : index === 0}
            status={node.kind === "site" ? siteStatus(node.status) : undefined}
            color={node.color === "none" ? undefined : (node.color as SiteColor)}
            meta={
              found
                ? t(found === "tags" ? "manager.inTags" : "manager.inNotes")
                : subsite
                  ? (["archived", "spam", "deleted"].includes(node.status) ? t(`subsite.state.${node.status}`) : undefined)
                  : node.multisite && node.subsites > 0
                    ? t("manager.subsites", { count: node.subsites })
                    : undefined
            }
            updates={node.kind === "host" ? node.updates : undefined}
            onSelect={(gesture) => choose(node.id, gesture)}
            onToggle={() => toggle(network ? openKey(node.id) : node.id)}
            itemId={node.id}
            onEdit={onEdit && !subsite ? () => onEdit(node.id) : undefined}
            editing={node.id === editingId}
            editLabel={t("manager.nameOf", { name: node.name })}
            onRename={(name) => onRename?.(node.id, name)}
            onCancelEdit={onCancelEdit}
            draggable={Boolean(onMove) && node.kind !== "site" && !subsite}
            dragging={drag?.ids.includes(node.id) ?? false}
            dropTarget={node.kind === "folder" && drag?.target === node.id}
            onDragStart={(event) => {
              const ids = moving(node.id);
              event.dataTransfer.effectAllowed = "move";
              event.dataTransfer.setData("text/plain", ids.map((id) => byId.get(id)?.name).join("\n"));
              // Several: a label says how many, by the pointer.
              if (ids.length > 1 && dragLabel.current) event.dataTransfer.setDragImage(dragLabel.current, 12, 16);
              follow({ id: node.id, ids, target: undefined });
            }}
            onDragEnd={() => follow(null)}
            onDragOver={(event) => over(event, targetOf(node))}
            onDrop={(event) => drop(event, targetOf(node))}
            onContextMenu={
              onMenu && !subsite
                ? (event) => {
                    event.preventDefault();
                    onMenu(node.id, { x: event.clientX, y: event.clientY });
                  }
                : undefined
            }
          />
        );
      })}
      {tagRows.map((r, index) =>
        r.tag ? (
          <TreeNode
            key={tagId(r.tag.name)}
            kind="tag"
            name={r.tag.name}
            depth={1}
            selected={selection.includes(tagId(r.tag.name))}
            meta={String(r.tag.sites)}
            itemId={tagId(r.tag.name)}
            onSelect={() => choose(tagId(r.tag!.name))}
            onEdit={onEdit && (() => onEdit(tagId(r.tag!.name)))}
            onContextMenu={
              onMenu &&
              ((event) => {
                event.preventDefault();
                onMenu(tagId(r.tag!.name), { x: event.clientX, y: event.clientY });
              })
            }
          />
        ) : (
          <TreeNode
            key={tagsGroupId}
            kind="folder"
            name={t("tags.label")}
            depth={0}
            expanded={filter.trim() !== "" || !collapsed.has(tagsGroupId)}
            tabbable={!hasSelection && visible.length === 0 && index === 0}
            itemId={tagsGroupId}
            onSelect={() => toggle(tagsGroupId)}
            onToggle={() => toggle(tagsGroupId)}
          />
        ),
      )}
      {carried.length > 1 && (
        <span ref={dragLabel} className={styles.dragLabel} aria-hidden="true">
          <span className={styles.dragIcon}>
            <Icon name={carriedKinds.has("host") ? "host" : "folder"} size={14} />
          </span>
          {carriedText}
        </span>
      )}
    </div>
  );
}

// The latest scan's status; a site never scanned shows the unknown ring.
function siteStatus(status: string): Status {
  return status === "ok" || status === "warn" || status === "danger" || status === "partial" ? status : "unknown";
}

