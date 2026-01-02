export default [
  {
    plugins: {
      security: (await import('eslint-plugin-security')).default
    },
    rules: {
      // Critical rules - block commits (code execution risks)
      "security/detect-child-process": "error",
      "security/detect-eval-with-expression": "error",
      "security/detect-non-literal-require": "error",
      // Important rules - warn only (may have false positives)
      "security/detect-object-injection": "warn",
      "security/detect-non-literal-regexp": "warn",
      "security/detect-unsafe-regex": "warn",
      "security/detect-buffer-noassert": "warn",
      "security/detect-no-csrf-before-method-override": "warn",
      "security/detect-non-literal-fs-filename": "warn",
      "security/detect-possible-timing-attacks": "warn",
      "security/detect-pseudoRandomBytes": "warn"
    }
  }
];
