import type { Plugin } from "vite";

// The window's Content Security Policy (ADR-0022): only what the app embeds. Images may also be
// data URIs, which is how add-ons give theirs; styles set through the CSSOM (React's style props)
// are allowed, a style attribute written in HTML is not.
export const policy = [
  "default-src 'none'",
  "script-src 'self'",
  "style-src 'self'",
  "img-src 'self' data:",
  "font-src 'self'",
  "connect-src 'self'",
  "base-uri 'none'",
  "form-action 'none'",
].join("; ");

/** Puts the policy first in the built page's head; the dev server, which reloads through inline
 *  scripts and a WebSocket, is left without it. */
export function contentSecurityPolicy(): Plugin {
  return {
    name: "vigie-content-security-policy",
    apply: "build",
    transformIndexHtml: () => [
      { tag: "meta", attrs: { "http-equiv": "Content-Security-Policy", content: policy }, injectTo: "head-prepend" },
    ],
  };
}
