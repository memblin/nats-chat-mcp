// Enforce Conventional Commits (feat:, fix:, chore:, docs:, refactor:, test:, …)
// on the commit-msg hook, matching this repo's existing history.
export default {
  extends: ["@commitlint/config-conventional"],
};
