// Cypress against the running Docker Compose stack (JT-95). Not run in CI, which
// has no stack; run locally with npm run test:stack (see tests/e2e/README.md).
const { defineConfig } = require('cypress');
module.exports = defineConfig({
  video: false,
  screenshotsFolder: 'cypress/screenshots/stack',
  defaultCommandTimeout: 10000,
  e2e: {
    baseUrl: 'http://localhost:5173',
    specPattern: '../tests/e2e/stack/**/*.cy.js',
    supportFile: false,
  },
});
