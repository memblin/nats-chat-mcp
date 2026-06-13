// ESLint flat config (ESLint 9). Lints TypeScript via typescript-eslint, and
// eslint-config-prettier is applied last so formatting is left entirely to
// Prettier (no rule conflicts between the two).
import js from "@eslint/js";
import tseslint from "typescript-eslint";
import prettier from "eslint-config-prettier";
import globals from "globals";

export default tseslint.config(
  { ignores: ["dist/**", "node_modules/**"] },
  js.configs.recommended,
  ...tseslint.configs.recommended,
  {
    languageOptions: {
      globals: { ...globals.node },
    },
  },
  prettier,
);
