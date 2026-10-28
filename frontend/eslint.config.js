import js from "@eslint/js";
import i18next from "eslint-plugin-i18next";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/", "bindings/"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    files: ["**/*.{ts,tsx}"],
    languageOptions: { globals: globals.browser },
    plugins: { "react-hooks": reactHooks },
    rules: reactHooks.configs.recommended.rules,
  },
  {
    // Every visible text comes from the translation catalogs.
    files: ["src/**/*.tsx"],
    ignores: ["src/**/*.stories.tsx", "src/**/*.test.tsx"],
    plugins: { i18next },
    rules: { "i18next/no-literal-string": "error" },
  },
);
