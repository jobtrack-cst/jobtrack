const { defineConfig } = require('cypress');
module.exports = defineConfig({
  video: false,
  e2e: {
    setupNodeEvents(on) {
      // Give single-frame report captures room without changing the test viewport.
      on('before:browser:launch', (browser, options) => {
        if (browser.name === 'electron' && process.env.JT101_CAPTURE === '1') {
          options.preferences.width = 1600;
          options.preferences.height = 2400;
        }
        return options;
      });
    },
    baseUrl: 'http://localhost:5173',
    specPattern: '../tests/e2e/**/*.cy.js',
    // Stack specs need the running Compose stack; they run with cypress.stack.config.cjs.
    excludeSpecPattern: '../tests/e2e/stack/**',
    supportFile: false,
  },
});
