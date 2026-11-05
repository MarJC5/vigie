import js from "@eslint/js";
import i18next from "eslint-plugin-i18next";
import reactHooks from "eslint-plugin-react-hooks";
import globals from "globals";
import tseslint from "typescript-eslint";

export default tseslint.config(
  { ignores: ["dist/", "storybook-static/", "bindings/"] },
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
  {
    // One component per field (ADR-0004, amendment of 2026-11-05): a view never draws its own.
    files: ["src/**/*.tsx"],
    ignores: [
      "src/components/{Checkbox,Radio,TextField,TextArea,InlineField,SelectField,SearchField}/*.tsx",
      "src/**/*.stories.tsx",
      "src/**/*.test.tsx",
    ],
    rules: {
      "no-restricted-syntax": [
        "error",
        {
          selector: "JSXOpeningElement[name.name=/^(input|select|textarea)$/]",
          message: "A field is drawn by its component: Checkbox, Radio, TextField, TextArea, InlineField, SelectField or SearchField.",
        },
      ],
    },
  },
);
