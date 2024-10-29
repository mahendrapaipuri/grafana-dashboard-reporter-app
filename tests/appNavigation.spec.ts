import { test, expect } from "./fixtures";
import { testIds } from "../src/components/testIds";
import { ROUTES } from "../src/constants";

test.describe("navigating app", () => {
  test("Status should render successfully", async ({ gotoPage, page }) => {
    // Seems like plugin page takes a while to load in Grafana v11.3.0+
    test.setTimeout(10000);
    
    await gotoPage(`/${ROUTES.Status}`);
    await expect(page.getByText("Plugin Health Check")).toBeVisible({timeout: 5000});
    await expect(page.getByTestId(testIds.Status.health)).toContainText("OK");
  });
});
