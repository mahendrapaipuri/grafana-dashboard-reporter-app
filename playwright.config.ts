import type { PluginOptions } from "@grafana/plugin-e2e";
import { defineConfig, devices } from "@playwright/test";
import { dirname } from "node:path";

const pluginE2eAuth = `${dirname(require.resolve("@grafana/plugin-e2e"))}/auth`;

/**
 * Read environment variables from file.
 * https://github.com/motdotla/dotenv
 */
// require('dotenv').config();

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
    grafanaAPICredentials: {
      user: "admin",
      password: "admin",
    },
    httpCredentials: {
      username: "admin",
      password: "admin",
    },

    /* Collect trace when retrying the failed test. See https://playwright.dev/docs/trace-viewer */
    trace: "on-first-retry",
    video: "retain-on-failure",
  },

  /* Configure projects for major browsers */
  projects: [
    // Login to Grafana with admin user and store the cookie on disk for use in other tests
    {
      name: "authenticateAdminUser",
      testDir: "./tests/setup",
      testMatch: [/.*auth\.setup\.ts/],
      use: {
        baseURL: `http://localhost:3080`,
        user: {
          user: "admin",
          password: "admin",
        },
      },
    },
    // Login to Grafana with new user with viewer role and store the cookie on disk for use in other tests
    {
      name: "createUserAndAuthenticate",
      testDir: "./tests/setup",
      testMatch: [/.*auth\.setup\.ts/],
      use: {
        baseURL: `https://localhost:3443`,
        user: {
          user: "viewer",
          password: "password",
          role: "Viewer",
        },
      },
    },
    // Login to Grafana with teamuser user and store the cookie on disk for use in other tests
    {
      name: "authenticateTeamUser",
      testDir: "./tests/setup",
      testMatch: [/.*auth\.setup\.ts/],
      use: {
        baseURL: `http://localhost:3080`,
        user: {
          user: "teamuser",
          password: "teamuser",
        },
      },
    },
    // Login to Grafana with normaluser user and store the cookie on disk for use in other tests
    {
      name: "authenticateNormalUser",
      testDir: "./tests/setup",
      testMatch: [/.*auth\.setup\.ts/],
      use: {
        baseURL: `https://localhost:3443`,
        user: {
          user: "normaluser",
          password: "normaluser",
        },
      },
    },
    // Plain without TLS and using admin user
    {
      name: "plainAdminUser",
      testIgnore: /(appReportTeamUser|appReportNormalUser)\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `http://localhost:3080`,
        storageState: "playwright/.auth/admin.json",
      },
      dependencies: ["authenticateAdminUser"],
    },
    // With TLS and using user with Viewer role
    {
      name: "tlsViewerUser",
      testIgnore: /(appConfig|appReportTeamUser|appReportNormalUser)\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `https://localhost:3443`,
        storageState: "playwright/.auth/viewer.json",
      },
      dependencies: ["createUserAndAuthenticate"],
    },
    // Plain without TLS and using teamuser
    {
      name: "plainTeamUser",
      testMatch: /appReportTeamUser\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `http://localhost:3080`,
        storageState: "playwright/.auth/teamuser.json",
      },
      dependencies: ["authenticateTeamUser"],
    },
     // With TLS and using normaluser
     {
      name: "tlsNormalUser",
      testMatch: /appReportNormalUser\.spec\.ts/,
      use: {
        ...devices["Desktop Chrome"],
        baseURL: `https://localhost:3443`,
        storageState: "playwright/.auth/normaluser.json",
      },
      dependencies: ["authenticateNormalUser"],
    },
  ],
});
