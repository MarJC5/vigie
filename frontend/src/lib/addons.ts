// What the Add-ons page shows of an add-on's description.
import type { AddonView } from "../../bindings/github.com/MarJC5/vigie/internal/app/models";

/** Where the contract of an add-on is documented. */
export const addonsDocs = "https://github.com/MarJC5/vigie/blob/main/docs/addons.md";

/** A text of an add-on in a language ("fr", "fr-CH"), else in English. */
export function textIn(text: { [lang: string]: string | undefined } | null | undefined, lang: string): string {
  const base = lang.split("-")[0] ?? "en";
  return text?.[base] || text?.en || "";
}

/** The program of an add-on. */
export const programOf = (addon: Pick<AddonView, "name">) => `vigie-${addon.name}`;

/** The values a run starts with: each setting's default. */
export function defaultsOf(addon: AddonView): Record<string, string> {
  return Object.fromEntries((addon.settings ?? []).map((s) => [s.id, s.default]));
}

/** Whether a run can start: every required setting has a value. */
export function canRun(addon: AddonView, values: Record<string, string>): boolean {
  return addon.kind === "source" && !addon.problem && (addon.settings ?? []).every((s) => !s.required || (values[s.id] ?? "") !== "");
}
