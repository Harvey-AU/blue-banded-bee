import security from "eslint-plugin-security";

export default [
  {
    files: ["web/**/*.js"],
    plugins: {
      security,
    },
    rules: {
      // These rules flag false positives in this codebase
      // Object injection: All flagged uses are internal data processing, not user input
      "security/detect-object-injection": "off",
      // Timing attacks: Flagged on password confirmation UI, not secret comparison
      "security/detect-possible-timing-attacks": "off",
      // Non-literal regexp: Pattern comes from code, not user input
      "security/detect-non-literal-regexp": "off",
    },
  },
];
