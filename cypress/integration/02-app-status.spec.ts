import { e2e } from "@grafana/e2e";
import { testIds } from "../../src/components/testIds";
import pluginJson from "../../src/plugin.json";

const { Status } = e2e.getSelectors(testIds);

describe("visiting status endpoint", () => {
  beforeEach(() => {
    cy.visit(`http://localhost:3000/a/${pluginJson.id}/status`);
  });

  it("should successfully call status", () => {
    // wait for page to successfully render
    Status.container().should("be.visible");
  });

  it("should successfully check health", () => {
    // wait for page to successfully render
    Status.container().should("be.visible");

    // wait for health check to be successful
    Status.health().should("be.visible").contains("OK");
  });
});
