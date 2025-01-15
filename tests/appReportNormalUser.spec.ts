import { test, expect } from "./fixtures";
import pluginJson from "../src/plugin.json";

// const fs = require("node:fs/promises");
// const { chromium } = require("playwright");

const dashboardUid = "be6xghyd5cf0gb";

test("should not be possible to generate report by a normal user without permissions", async ({ request }, testInfo) => {
  // Set larger timeout
  test.setTimeout(60000);

  const report = await request.get(
    `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}`
  );

  // This case does not have necessary feature flags enabled and thus
  // report generation should pass. In rest of the cases, it should fail
  // due to access control
  if (
    (process.env.GRAFANA_VERSION === "10.4.7" &&
    process.env.GF_FEATURE_TOGGLES_ENABLE === "externalServiceAccounts") ||
    process.env.GRAFANA_VERSION === "11.3.0-security-01"
  ) {
    expect(report.ok()).toBeTruthy();
  } else {
    expect(report.ok()).toBeFalsy();
  }
});
