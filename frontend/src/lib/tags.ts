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
