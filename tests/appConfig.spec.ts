import { test, expect } from "./fixtures";
import { testIds } from "../src/components/testIds";

test("should be possible to save app configuration", async ({
  appConfigPage,
  page,
}) => {
  const saveButton = page.getByRole("button", { name: /Save settings/i });

  // Check if all config parameters are visible
  for (const t in testIds.appConfig) {
    await expect(page.getByTestId(testIds.appConfig[t])).toBeVisible;
  }

  // reset the configured secret
  const resetButton = page.getByRole("button", { name: /reset/i });
  if ((await resetButton.count()) > 0) {
    await resetButton.click();
  }

  // enter some valid values
  await page
    .getByRole("textbox", { name: "Service Account Token" })
    .fill("secret-api-key");
  await page.getByLabel("Grid").click();
  await page.getByLabel("Expand section Additional Settings").click();
  await page.getByLabel("Maximum Render Workers").clear();
  await page.getByLabel("Maximum Render Workers").fill("3");

  // listen for the server response on the saved form
  const saveResponse = appConfigPage.waitForSettingsResponse();

  await saveButton.click();
  await expect(saveResponse).toBeOK();

  // Reset the configured token and save settings
  await resetButton.click();
  await saveButton.click();
  await expect(saveResponse).toBeOK();
});
