// Every component has its stories: they are its gallery entry, its documentation and its tests.
import { existsSync, readdirSync } from "node:fs";
import { describe, expect, it } from "vitest";

const root = new URL("../src/components/", import.meta.url);
const components = readdirSync(root, { withFileTypes: true }).filter((d) => d.isDirectory()).map((d) => d.name);

describe("components", () => {
  it("exist", () => {
    expect(components.length).toBeGreaterThan(0);
  });

  it.each(components)("%s has a story file", (name) => {
    expect(existsSync(new URL(`${name}/${name}.stories.tsx`, root))).toBe(true);
  });
});
