import { describe, expect, it } from "vitest";

import { since } from "./time";

const now = new Date("2026-10-29T09:00:00Z");

describe("since", () => {
  it.each([
    ["2026-10-29T08:59:40Z", "en", "now"],
    ["2026-10-29T08:57:00Z", "en", "3 minutes ago"],
    ["2026-10-29T06:00:00Z", "en", "3 hours ago"],
    ["2026-10-27T09:00:00Z", "en", "2 days ago"],
    ["2026-10-29T08:57:00Z", "fr", "il y a 3 minutes"],
  ])("%s in %s is %s", (when, lang, want) => {
    expect(since(when, lang, now)).toBe(want);
  });
});
