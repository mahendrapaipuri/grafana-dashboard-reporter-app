import { e2e } from "@grafana/e2e";
import { testIds } from "../../src/components/testIds";
import pluginJson from "../../src/plugin.json";

const { Panel } = e2e.getSelectors(testIds);

const dashboardUid = "e472bbf0-140c-4852-a74b-1a4c32202659";
const queryParams = `from=now-5m&to=now&var-testvar0=All&var-testvar1=foo&var-testvar2=1&layout=grid&excludePanelID=12&excludePanelID=28&excludePanelID=10&includePanelID=10&dashboardMode=full`;

describe("visiting test dashboard", () => {
  beforeEach(() => {
    e2e.flows.openDashboard({
      uid: dashboardUid,
    });
  });

  it("should display test panels", () => {
    e2e.components.Panels.Panel.title("Panel 11 Var 0").should("be.visible");
    Panel.container().should("be.visible");
  });

  it("should display dashboard report link", () => {
    e2e.components.DashboardLinks.link()
      .should("be.visible")
      .should((links) => {
        expect(links).to.have.length.greaterThan(0);

        for (let index = 0; index < links.length; index++) {
          expect(Cypress.$(links[index]).attr("href")).contains(
            `dashUid=${dashboardUid}`
          );
        }
      });
  });

  it("should successfully create pdf", () => {
    cy.request({
      url: `http://localhost:3000/api/plugins/${pluginJson.id}/resources/report?dashUid=${dashboardUid}&${queryParams}`,
      timeout: 60000,
    }).then((response) => {
      expect(response.status).to.eq(200);
      expect(response.headers["content-type"]).to.eq("application/pdf");
    });
  });
});
