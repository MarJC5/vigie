// Raw colors live in tokens.css only; every other stylesheet uses the --vg-* variables. No
// accent-color: the checkboxes and radios are drawn by Vigie.
export default {
  rules: {
    "color-no-hex": true,
    "color-named": "never",
    // Checkboxes and radios are drawn by Vigie, the same in every engine (ADR-0004).
    "property-disallowed-list": ["accent-color"],
  },
  overrides: [
    {
      files: ["src/styles/tokens.css"],
      rules: {
        "color-no-hex": null,
        "color-named": null,
      },
    },
  ],
};
