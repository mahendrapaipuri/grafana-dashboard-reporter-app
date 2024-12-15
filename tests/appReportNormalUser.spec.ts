import { test, expect } from "./fixtures";
import pluginJson from "../src/plugin.json";

// const fs = require("node:fs/promises");
// const { chromium } = require("playwright");

const dashboardUid = "be6xghyd5cf0gb";

test("should not be possible to generate report by a normal user without permissions", async ({ request }) => {
  // Set larger timeout
  test.setTimeout(60000);

  const report = await request.get(
    `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}`
  );

  expect(report.ok()).toBeFalsy();
});
