import { e2e } from '@grafana/e2e';
import { testIds } from '../../src/components/testIds';
import pluginJson from '../../src/plugin.json';

const { appConfig } = e2e.getSelectors(testIds);

describe('configurating app', () => {
  beforeEach(() => {
    cy.visit(`http://localhost:3000/plugins/${pluginJson.id}`);
  });

  it('should be successfully configured', () => {
    // wait for page to successfully render
    appConfig.container().should('be.visible');

    // enter some configuration values
    appConfig.maxWorkers().clear();
    appConfig.maxWorkers().type('3');
    appConfig.texTemplate().type(`test template
multiline input`);
    appConfig.submit().click();

    // make sure it got updated successfully
    appConfig.maxWorkers().should('have.value', '3');
    appConfig.texTemplate().should('have.value', `test template
multiline input`);
  });
});
