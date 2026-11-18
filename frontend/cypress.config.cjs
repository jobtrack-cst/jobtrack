const { defineConfig } = require('cypress');
module.exports = defineConfig({
  video: false,
  e2e: {
    baseUrl: 'http://localhost:5173',
    specPattern: '../tests/e2e/**/*.cy.js',
    // Stack specs need the running Compose stack; they run with cypress.stack.config.cjs.
    excludeSpecPattern: '../tests/e2e/stack/**',
    supportFile: false,
  },
});
