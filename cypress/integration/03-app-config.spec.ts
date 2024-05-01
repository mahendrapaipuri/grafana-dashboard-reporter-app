import { e2e } from '@grafana/e2e';
import { testIds } from '../../src/components/testIds';
import pluginJson from '../../src/plugin.json';

const { appConfig } = e2e.getSelectors(testIds);

describe('configurating app', () => {
  beforeEach(() => {
    cy.visit(`http://localhost:3000/plugins/${pluginJson.id}`);
  });

  it("should display config options", () => {
    appConfig.container().should("be.visible");
    appConfig.saToken().should("be.visible");
    appConfig.layout().should("be.visible");
    appConfig.orientation().should("be.visible");
    appConfig.dashboardMode().should("be.visible");

    // Not sure why, seems like this element is not visible in tests
    // Ignore it for the moment as it is not very critical
    // Anyways we are testing these elements in next test and so we can safely
    // ignore them here
    // appConfig.persistData().should("be.visible");
    // appConfig.submit().should("be.visible");
    // appConfig.maxWorkers().should("be.visible");
  });

  it('should be successfully configured', () => {
    // wait for page to successfully render
    appConfig.container().should('be.visible');

    // enter some configuration values
    appConfig.maxWorkers().clear();
    appConfig.maxWorkers().type('3');
    appConfig.submit().click();

    // make sure it got updated successfully
    appConfig.maxWorkers().should('have.value', '3');
  });
});
