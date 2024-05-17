import { test, expect } from "./fixtures";
import pluginJson from "../src/plugin.json";

// const fs = require("node:fs/promises");
// const { chromium } = require("playwright");

const dashboardUid = "fdlwjnyim1la8f";
const queryParams = `from=now-5m&to=now&var-testvar0=All&var-testvar1=foo&var-testvar2=1&layout=grid&excludePanelID=5&dashboardMode=full`;

test("should be possible to generate report", async ({ request }) => {
  // Set larger timeout
  test.setTimeout(60000);

  const report = await request.get(
    `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}&${queryParams}`
  );
  expect(report.ok()).toBeTruthy();

  // const browser = await chromium.launch({
  //   headless: true,
  //   acceptDownloads: true,
  //   AlwaysOpenPdfExternally: true,
  // });
  // const context = await browser.newContext();
  // const page = await context.newPage();

  // // let promise = page.waitForEvent("download");
  // // try {
  // //   await page.goto(
  // //     `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}&${queryParams}`,
  // //     { waitUntil: "networkidle" }
  // //   );
  // // } catch (e) {}
  // // let download = await promise;
  // await page.goto(
  //   `/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}&${queryParams}`,
  //   { waitUntil: "networkidle" }
  // );

  // const imageName = "launcher-category.png";
  // expect(await page.screenshot()).toMatchSnapshot(imageName.toLowerCase());

  // await browser.close();
});
