import type { TagView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";
import { fold } from "./text";

/** The most tags offered while one is typed. */
const maxOffered = 6;

/** What a site's tag field offers for what is typed (ADR-0025): the tags in use that hold it and
 * the site has not, the closest first; then, unless one of them is what was typed, a new tag. */
export function tagChoices(used: TagView[], current: string[], typed: string): Array<{ name: string; sites: number; isNew: boolean }> {
  const query = fold(typed.trim());
  if (!query) return [];
  const on = new Set(current.map(fold));
  const offered = used
    .filter((u) => !on.has(fold(u.name)) && fold(u.name).includes(query))
    .sort((a, b) => Number(!fold(b.name).startsWith(query)) - Number(!fold(a.name).startsWith(query)) || b.sites - a.sites)
    .slice(0, maxOffered)
    .map((u) => ({ name: u.name, sites: u.sites, isNew: false }));
  const known = used.some((u) => fold(u.name) === query) || on.has(query);
  return known ? offered : [...offered, { name: typed.trim().replace(/\s+/g, " "), sites: 0, isNew: true }];
}

/** The Tags group of the site list, folded like a folder. */
export const tagsGroupId = "tags";

/** The ID of a tag's page: the Go side takes "tag:" and the tag as a node (app.TagPrefix). */
export function tagId(tag: string): string {
  return `tag:${tag}`;
}

/** The tag of a tag page's ID; undefined for any other ID. */
export function tagOf(id: string | undefined): string | undefined {
  return id?.startsWith("tag:") ? id.slice(4) : undefined;
}

/** Whether a site has a tag, case and accents aside. */
export function hasTag(tags: string[] | null | undefined, tag: string): boolean {
  return (tags ?? []).some((t) => fold(t) === fold(tag));
}

/** Where a site is, from its path: its folders and host, "Clients / Shared hosting". */
export function whereOf(path: string): string {
  return path.split("/").slice(0, -1).join(" / ");
}

/** How many sites, hosts and folders a tag's sites are spread over, from their paths. */
export function spread(paths: string[]): { sites: number; hosts: number; folders: number } {
  const hosts = new Set(paths.map((p) => p.split("/").slice(0, -1).join("/")));
  const folders = new Set([...hosts].map((h) => h.split("/").slice(0, -1).join("/")).filter(Boolean));
  return { sites: paths.length, hosts: hosts.size, folders: folders.size };
}
