import { test, expect } from "./fixtures";
import pluginJson from "../src/plugin.json";

// const fs = require("node:fs/promises");
// const { chromium } = require("playwright");

const dashboardUid = "fdlwjnyim1la8f";
const queryParams = `from=now-5m&to=now&var-testvar0=All&var-testvar1=foo&var-testvar2=1&layout=grid&excludePanelID=5&dashboardMode=full`;

test("should be possible to generate report", async ({ request }, testInfo) => {
  // Set larger timeout
  test.setTimeout(60000);

  const report = await request.get(
    `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}&${queryParams}`
  );

  // TLS case will attempt to create a report by a user without View permission
  // on dashboard except for the case where Grafana 10.4.7 and not using appropriate
  // feature toogles. In the exceptional case, the test should pass
  if (
    testInfo.project.name === "tlsViewerUser" &&
    process.env.GRAFANA_VERSION !== "10.4.7" &&
    process.env.GF_FEATURE_TOGGLES_ENABLE !== "externalServiceAccounts"
  ) {
    expect(report.ok()).toBeFalsy();
  } else {
    expect(report.ok()).toBeTruthy();
  }
});
