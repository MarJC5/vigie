// The built page carries the window's Content Security Policy (ADR-0022), and the policy loads
// nothing from outside the app.
import { readFileSync } from "node:fs";
import { describe, expect, it } from "vitest";

import { contentSecurityPolicy, policy } from "../csp";

describe("Content Security Policy", () => {
  it("is added to the built page only, first in its head", () => {
    const plugin = contentSecurityPolicy();
    expect(plugin.apply).toBe("build");
    const transform = plugin.transformIndexHtml as () => Array<{ tag: string; attrs: Record<string, string>; injectTo: string }>;
    expect(transform()).toEqual([
      { tag: "meta", attrs: { "http-equiv": "Content-Security-Policy", content: policy }, injectTo: "head-prepend" },
    ]);
  });

  it("allows the app's own files only, and data URIs for images", () => {
    const directives = Object.fromEntries(policy.split("; ").map((d) => [d.split(" ")[0], d.split(" ").slice(1)]));
    expect(directives["default-src"]).toEqual(["'none'"]);
    for (const name of ["script-src", "style-src", "font-src", "connect-src"]) expect(directives[name]).toEqual(["'self'"]);
    expect(directives["img-src"]).toEqual(["'self'", "data:"]);
    expect(policy).not.toMatch(/unsafe|https?:|\*/);
  });

  it("is in the page the build wrote, when there is one", () => {
    let page: string;
    try {
      page = readFileSync(new URL("../dist/index.html", import.meta.url), "utf8");
    } catch {
      return; // not built yet: the end-to-end test checks the page served
    }
    // Vite writes the quotes of the policy as HTML entities.
    expect(page.replaceAll("&#39;", "'")).toContain(`<meta http-equiv="Content-Security-Policy" content="${policy}">`);
  });
});
