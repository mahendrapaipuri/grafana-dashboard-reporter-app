import { test, expect } from "./fixtures";
import { testIds } from "../src/components/testIds";
import { ROUTES } from "../src/constants";

test.describe("navigating app", () => {
  test("Status should render successfully", async ({ gotoPage, page }) => {
    await gotoPage(`/${ROUTES.Status}`);
    await expect(page.getByText("Plugin Health Check")).toBeVisible();
    await expect(page.getByTestId(testIds.Status.health)).toContainText("OK");
  });
});
