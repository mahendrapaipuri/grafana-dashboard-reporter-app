import type { PluginOptions } from "@grafana/plugin-e2e";
import { defineConfig, devices } from "@playwright/test";
import { dirname } from "node:path";

const pluginE2eAuth = `${dirname(require.resolve("@grafana/plugin-e2e"))}/auth`;

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

// Auth header
const btoa = (str: string) => Buffer.from(str).toString("base64");
const credentialsBase64 = btoa(`admin:admin`);

/**
 * See https://playwright.dev/docs/test-configuration.
 */
export default defineConfig<PluginOptions>({
  testDir: "./tests",
  /* Run tests in files in parallel */
  fullyParallel: true,
  /* Fail the build on CI if you accidentally left test.only in the source code. */
  forbidOnly: !!process.env.CI,
  /* Retry on CI only */
  retries: process.env.CI ? 2 : 0,
  /* Opt out of parallel tests on CI. */
  workers: process.env.CI ? 1 : undefined,
  /* Reporter to use. See https://playwright.dev/docs/test-reporters */
  reporter: "html",
  /* Shared settings for all the projects below. See https://playwright.dev/docs/api/class-testoptions. */
  use: {
    /* Base URL to use in actions like `await page.goto('/')`. */
    // baseURL: "http://localhost:3080",

    /* Context options */
    viewport: { width: 1920, height: 1200 },

    /* Ignore certificate errors */
    ignoreHTTPSErrors: true,

    /* Auth */
    extraHTTPHeaders: {
      Authorization: `Basic ${credentialsBase64}`,
    },

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",
    video: "retain-on-failure",
  },

  /* Configure projects for major browsers */
  projects: [
    // Plain without TLS
    {
      name: "plain",
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `http://localhost:3080`,
      },
    },
    // With TLS
    {
      name: "tls",
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `https://localhost:3443`,
      },
    },
  ],
});
