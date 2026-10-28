import { type KeyboardEvent, useMemo, useRef, useState } from "react";
import { useTranslation } from "react-i18next";

import type { Node } from "../../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { type SiteColor, TreeNode } from "../TreeNode/TreeNode";
import styles from "./SiteTree.module.css";

export interface SiteTreeProps {
  label: string;
  /** Depth first, parents before their children, as SiteManager.Tree returns them. */
  nodes: Node[];
  selectedId?: string;
  onSelect: (id: string) => void;
}

/** The site manager tree: folders and hosts fold, arrow keys move between visible items. */
export function SiteTree({ label, nodes, selectedId, onSelect }: SiteTreeProps) {
  const { t } = useTranslation();
  const [collapsed, setCollapsed] = useState<Set<string>>(new Set());
  const ref = useRef<HTMLDivElement>(null);

  const visible = useMemo(() => {
    const byId = new Map(nodes.map((n) => [n.id, n]));
    const depthOf = (n: Node): number => (n.parentId ? depthOf(byId.get(n.parentId)!) + 1 : 0);
    const hidden = (n: Node): boolean => {
      for (let p = byId.get(n.parentId); p; p = byId.get(p.parentId)) if (collapsed.has(p.id)) return true;
      return false;
    };
    return nodes.filter((n) => !hidden(n)).map((n) => ({ node: n, depth: depthOf(n) }));
  }, [nodes, collapsed]);

  const toggle = (id: string) =>
    setCollapsed((current) => {
      const next = new Set(current);
      if (!next.delete(id)) next.add(id);
      return next;
    });

  const onKeyDown = (event: KeyboardEvent) => {
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
    <div ref={ref} role="tree" aria-label={label} className={styles.tree} onKeyDown={onKeyDown}>
      {visible.map(({ node, depth }, index) => (
        <TreeNode
          key={node.id}
          kind={node.kind as "folder" | "host" | "site"}
          name={node.name}
          depth={depth}
          expanded={node.kind === "site" ? undefined : !collapsed.has(node.id)}
          selected={node.id === selectedId}
          tabbable={hasSelection ? undefined : index === 0}
          status={node.kind === "site" ? "unknown" : undefined}
          color={node.color === "none" ? undefined : (node.color as SiteColor)}
          meta={node.multisite ? t("manager.subsites", { count: node.subsites }) : undefined}
          onSelect={() => onSelect(node.id)}
          onToggle={() => toggle(node.id)}
        />
      ))}
    </div>
  );
}
