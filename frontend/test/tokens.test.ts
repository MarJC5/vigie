// Contrast of every token pair the interface relies on, in both themes, read from tokens.css
// (WCAG 2.2: 4.5:1 for text, 3:1 for icons, focus rings and control borders).
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

const css = readFileSync(new URL("../src/styles/tokens.css", import.meta.url), "utf8");

/** Variables declared in the first block whose selector starts with `selector`. */
function block(selector: string): Record<string, string> {
  const start = css.indexOf(selector);
  if (start < 0) throw new Error(`no block ${selector}`);
  const body = css.slice(css.indexOf("{", start) + 1, css.indexOf("}", start));
  const vars: Record<string, string> = {};
  for (const m of body.matchAll(/--vg-([a-z0-9-]+):\s*([^;]+);/g)) vars[m[1]!] = m[2]!.trim();
  return vars;
}

const themes = {
  dark: block(":root,\n[data-theme=\"dark\"]"),
  light: block("[data-theme=\"light\"]"),
};

function luminance(hex: string): number {
  const m = /^#([0-9a-f]{6})$/i.exec(hex);
  if (!m) throw new Error(`not a 6-digit hex color: ${hex}`);
  const [r, g, b] = [0, 2, 4].map((i) => {
    const c = parseInt(m[1]!.slice(i, i + 2), 16) / 255;
    return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * r! + 0.7152 * g! + 0.0722 * b!;
}

function ratio(a: string, b: string): number {
  const [la, lb] = [luminance(a), luminance(b)];
  return (Math.max(la, lb) + 0.05) / (Math.min(la, lb) + 0.05);
}

const surfaces = [
  "surface-rail",
  "surface-main",
  "surface-pane",
  "surface-row",
  "surface-sunken",
  "surface-raised",
  "surface-hover",
  "surface-selected",
];
const pairs: Array<[string, string[], number]> = [
  ["text", surfaces, 4.5],
  ["text-muted", surfaces, 4.5],
  ["accent-text", ["surface-main", "surface-pane", "surface-row", "surface-raised", "surface-selected"], 4.5],
  ["text-on-accent", ["accent", "accent-hover"], 4.5],
  ["focus", ["surface-main", "surface-pane", "surface-row"], 3],
  ["border-strong", ["surface-main", "surface-raised"], 3],
];

describe.each(Object.entries(themes))("%s theme", (_name, vars) => {
  it.each(pairs)("%s against %j reaches %d:1", (fg, backgrounds, min) => {
    for (const bg of backgrounds) {
      const value = ratio(vars[fg]!, vars[bg]!);
      expect(value, `${fg} ${vars[fg]} on ${bg} ${vars[bg]}`).toBeGreaterThanOrEqual(min);
    }
  });
});

describe("tokens.css", () => {
  it("declares the same tokens in both themes", () => {
    expect(Object.keys(themes.light).sort()).toEqual(Object.keys(themes.dark).sort());
  });

  it("repeats the light theme identically for the system preference", () => {
    const media = css.slice(css.indexOf("@media (prefers-color-scheme: light)"));
    const vars: Record<string, string> = {};
    for (const m of media.slice(0, media.indexOf("}\n}")).matchAll(/--vg-([a-z0-9-]+):\s*([^;]+);/g)) vars[m[1]!] = m[2]!.trim();
    expect(vars).toEqual(themes.light);
  });
});
