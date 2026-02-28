import { test, expect } from "./fixtures";
import pluginJson from "../src/plugin.json";

// const fs = require("node:fs/promises");
// const { chromium } = require("playwright");

const dashUid = "fe6xfyecry03ke";

test("should be possible to generate report by a team user using folder permissions", async ({ request }) => {
  // Set larger timeout
  test.setTimeout(60000);

  const report = await request.get(
    `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashUid}`
  );

  expect(report.ok()).toBeTruthy();
});
