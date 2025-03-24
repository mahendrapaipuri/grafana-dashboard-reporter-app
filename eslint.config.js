const grafanaConfig = require("@grafana/eslint-config/flat");

/**
 * @type {Array<import('eslint').Linter.Config>}
 */
module.exports = [
  {
    ignores: [".github", ".yarn", "**/build/", "**/compiled/", "**/dist/", ".gitignore"],
  },
  grafanaConfig,
];
