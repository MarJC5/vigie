// The design system relies on two lint rules: no raw color outside tokens.css, and no text written
// in a component. These tests prove the rules are active, so a configuration change cannot silently
// drop them.
import { ESLint } from "eslint";
import stylelint from "stylelint";
import { describe, expect, it } from "vitest";

describe("stylelint", () => {
  const lint = (code: string, codeFilename: string) =>
    stylelint.lint({ code, codeFilename, configFile: "stylelint.config.js" });

  it("rejects a hex color in a component stylesheet", async () => {
    const result = await lint(".button { color: #3858e9; }", "src/components/Button.module.css");
    expect(result.errored).toBe(true);
  });

  it("rejects a named color in a component stylesheet", async () => {
    const result = await lint(".button { color: red; }", "src/components/Button.module.css");
    expect(result.errored).toBe(true);
  });

  it("accepts token variables in a component stylesheet", async () => {
    const result = await lint(".button { color: var(--vg-accent); }", "src/components/Button.module.css");
    expect(result.errored).toBe(false);
  });

  it("accepts raw colors in tokens.css only", async () => {
    const result = await lint(":root { --vg-accent: #3858e9; }", "src/styles/tokens.css");
    expect(result.errored).toBe(false);
  });
});

describe("eslint", () => {
  const eslint = new ESLint();
  const errors = async (code: string) => {
    const [result] = await eslint.lintText(code, { filePath: "src/components/Fixture.tsx" });
    return (result?.messages ?? []).filter((m) => m.severity === 2).map((m) => m.ruleId);
  };

  // Components are PascalCase: the rule skips names written in capitals only, which it takes for
  // constants (const SITE_LIST = "..."), so a component must never be named like one.
  it("rejects text written in JSX", async () => {
    expect(await errors("export const Button = () => <p>Scan now</p>;\n")).toContain("i18next/no-literal-string");
  });

  it("rejects text in a function component", async () => {
    expect(await errors("export function Tabs() { return <span>Overview</span>; }\n")).toContain("i18next/no-literal-string");
  });

  it("accepts text coming from the translation function", async () => {
    const code = "declare const t: (k: string) => string;\nexport const Button = () => <p>{t(\"site.scan\")}</p>;\n";
    expect(await errors(code)).toEqual([]);
  });
});
