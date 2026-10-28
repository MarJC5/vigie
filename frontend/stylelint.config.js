// Raw colors live in tokens.css only; every other stylesheet uses the --vg-* variables.
export default {
  rules: {
    "color-no-hex": true,
    "color-named": "never",
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
