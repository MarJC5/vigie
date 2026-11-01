// The subsites of a network (ADR-0029): their place in the site list, and where a plugin or theme
// is active on the network.
import type { ExtensionView, Node, SubsiteView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

/** The ID of a subsite's page in the site list; not stored, as a tag's page is not. */
export function subsiteId(siteId: string, blogId: number): string {
  return `subsite:${siteId}:${blogId}`;
}

/** The network and subsite of a subsite's page ID; undefined for any other ID. */
export function subsiteOf(id: string | undefined): { siteId: string; blogId: number } | undefined {
  const m = /^subsite:(.+):(\d+)$/.exec(id ?? "");
  return m ? { siteId: m[1]!, blogId: Number(m[2]) } : undefined;
}

/** The key, among the folded items kept, of a network unfolded: networks are folded by default. */
export function openKey(siteId: string): string {
  return `open:${siteId}`;
}

/** The site list's nodes with each network's subsites right under it, as nodes of kind "subsite"
 * whose status is the subsite's state (public, hidden, archived, spam, deleted). */
export function withSubsites(nodes: Node[]): Node[] {
  return nodes.flatMap((n) => [
    n,
    ...(n.subsiteList ?? []).map(
      (s): Node => ({
        id: subsiteId(n.id, s.blogId), parentId: n.id, kind: "subsite", name: s.label, color: "none", ownColor: "none",
        path: s.label === "/" ? n.path : `${n.path}${s.label.startsWith("/") ? "" : "/"}${s.label}`,
        notes: "", tags: [], multisite: false, subsites: 0, subsiteList: [], status: s.state, updates: 0,
      }),
    ),
  ]);
}

/** Where a plugin or theme is active on a network: network-wide, on some subsites, or nowhere;
 * undefined on a single site, or when the scan did not read the subsites. */
export function activation(e: ExtensionView): { kind: "network" | "some" | "nowhere"; sites: number[] } | undefined {
  if (e.subsites == null) return undefined;
  if (e.kind === "plugin" && e.status === "active-network") return { kind: "network", sites: [] };
  return e.subsites.length > 0 ? { kind: "some", sites: e.subsites } : { kind: "nowhere", sites: [] };
}

/** Subsites named in a sentence: "/alpha et /beta". */
export function listOf(labels: string[], lang: string): string {
  return new Intl.ListFormat(lang, { type: "conjunction" }).format(labels);
}

/** A network's subsites by their ID, as findingText names them. */
export function labelsOf(subsites: SubsiteView[] | null | undefined): Record<string, string> {
  return Object.fromEntries((subsites ?? []).map((s) => [String(s.blogId), s.label]));
}
