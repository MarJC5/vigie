// Several items selected in the site list (ADR-0034): what they hold, and what a delete of them removes.
import type { Node } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

/** The sites the items hold, each once, in the tree's order. */
export function sitesOf(ids: string[], nodes: Node[]): Node[] {
  const chosen = new Set(ids);
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const held = (n: Node) => {
    for (let p: Node | undefined = n; p; p = byId.get(p.parentId)) if (chosen.has(p.id)) return true;
    return false;
  };
  return nodes.filter((n) => n.kind === "site" && held(n));
}

/** How many folders, hosts and sites are among the items. */
export function kindsOf(ids: string[], nodes: Node[]): { folders: number; hosts: number; sites: number } {
  const chosen = new Set(ids);
  const count = (kind: string) => nodes.filter((n) => n.kind === kind && chosen.has(n.id)).length;
  return { folders: count("folder"), hosts: count("host"), sites: count("site") };
}

/** What a delete of the items removes: every site and host, and each folder left empty once the
 others go; the folders that stay hold something not selected. going lists the items to delete;
 shown, the ones not under another that goes. */
export function deletion(ids: string[], nodes: Node[]): { going: Node[]; shown: Node[]; staying: Node[] } {
  const chosen = new Set(ids);
  const byId = new Map(nodes.map((n) => [n.id, n]));
  const gone = new Set(nodes.filter((n) => chosen.has(n.id) && (n.kind === "site" || n.kind === "host")).map((n) => n.id));
  const depth = (n: Node): number => (n.parentId ? depth(byId.get(n.parentId)!) + 1 : 0);
  const folders = nodes.filter((n) => chosen.has(n.id) && n.kind === "folder").sort((a, b) => depth(b) - depth(a));
  const staying: Node[] = [];
  for (const f of folders) {
    if (nodes.every((n) => n.parentId !== f.id || n.kind === "site" || gone.has(n.id))) gone.add(f.id);
    else staying.push(f);
  }
  const going = nodes.filter((n) => gone.has(n.id));
  const shown = going.filter((n) => !gone.has(n.parentId));
  return { going, shown, staying: nodes.filter((n) => staying.includes(n)) };
}

/** The text of folders and hosts moved together, with their count: hosts, folders or items. */
export function carriedKey(kinds: (string | undefined)[]): string {
  const set = new Set(kinds);
  if (set.size > 1) return "manager.dragItems";
  return set.has("host") ? "manager.dragHosts" : "manager.dragFolders";
}
