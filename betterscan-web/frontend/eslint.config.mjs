import { defineConfig, globalIgnores } from "eslint/config";
import nextVitals from "eslint-config-next/core-web-vitals";
import nextTs from "eslint-config-next/typescript";

const eslintConfig = defineConfig([
  ...nextVitals,
  ...nextTs,
  // Override default ignores of eslint-config-next.
  globalIgnores([
    // Default ignores of eslint-config-next:
    ".next/**",
    "out/**",
    "build/**",
    "next-env.d.ts",
  ]),
  {
    rules: {
      // Auth.js callbacks use loose shapes; keep them typed loosely at the boundary.
      "@typescript-eslint/no-explicit-any": "off",
      // Client pages legitimately load remote data after session status settles.
      "react-hooks/set-state-in-effect": "off",
      // Prefer router elsewhere; some redirects are intentional full navigations.
      "@next/next/no-location-assign-relative-destination": "warn",
      "react-hooks/exhaustive-deps": "warn",
    },
  },
]);

export default eslintConfig;
