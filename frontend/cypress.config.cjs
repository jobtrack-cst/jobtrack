const { defineConfig } = require('cypress');
module.exports = defineConfig({
  video: false,
  e2e: {
    baseUrl: 'http://localhost:5173',
    specPattern: '../tests/e2e/**/*.cy.js',
    supportFile: false,
  },
});
