import { openApplications, settleHome } from './home';
const gateway = 'http://localhost:8000';
// Keep the applicant app-bar photo request independent of a running gateway.
beforeEach(() => cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} }));
beforeEach(() => {
  cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: '30000000-0000-4000-8000-000000000001', name: 'Thimphu' }] });
});
function locationSelect() {
  return cy.contains('label', /^Location$/).parent().find('[role="combobox"]');
}
function selectLocation(name) {
  // MUI's non-native select is a div; Cypress does not wait on aria-disabled.
  locationSelect().should('be.visible').and('not.have.attr', 'aria-disabled', 'true').click();
  locationSelect().should('have.attr', 'aria-expanded', 'true');
  cy.get('[role="listbox"]').should('be.visible')
    .contains('[role="option"]', new RegExp(`^${Cypress._.escapeRegExp(name)}$`)).click();
  locationSelect().should('have.attr', 'aria-expanded', 'false');
}
const applicantId = '11111111-1111-4111-8111-111111111111';
const applicationId = '44444444-4444-4444-8444-444444444444';
const offerId = '55555555-5555-4555-8555-555555555555';
function login(role = 'ROLE_APPLICANT') {
  const user = { userId: applicantId, email: 'pema@example.test', fullName: 'Pema', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'workflow-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every protected workflow request must pass through the shared gateway client.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer workflow-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', role === 'ROLE_APPLICANT' ? '/applicant' : role === 'ROLE_EMPLOYER' ? '/employer' : '/admin');
  settleHome();
}
function navigate(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
// Applicant apply, applications and offers moved to applicant-pages.cy.js (JT-112). The
// employer applications, offers, messages and JT-108 offer discovery moved with the JT-111
// application page to employer-pages.cy.js; job alerts and route checks stay here.
describe('JT-78 job alerts and routes', () => {
  it('creates and deletes an exact-location subscription; empty filters cannot submit', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    login(); cy.contains('a', 'Job alerts').click();
    cy.contains('button', 'Create subscription').should('be.disabled');
    selectLocation('Thimphu');
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [], tagIds: [], location: 'Thimphu' });
      req.reply({ jobAlert: { alertId: offerId, location: 'Thimphu' } });
    }).as('create');
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [{ alertId: offerId, location: 'Thimphu' }] });
    cy.contains('button', 'Create subscription').click(); cy.wait('@create');
    cy.contains('Subscription created.').should('be.visible');
    cy.contains('Location: Thimphu').should('be.visible');
    cy.intercept('DELETE', `${gateway}/v1/job-alerts/${offerId}`, { body: {} }).as('delete');
    cy.contains('button', 'Delete subscription').click();
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.contains('button', 'Confirm deletion').click(); cy.wait('@delete');
    cy.contains('No subscriptions').should('be.visible');
  });
  it('restricts feature routes by role', () => {
    login('ROLE_EMPLOYER'); navigate('/applicant/job-alerts');
    cy.location('pathname').should('eq', '/employer');
    navigate('/applicant/apply'); cy.location('pathname').should('eq', '/employer');
  });
  it('redirects expired workflow sessions to login on protected 401', () => {
    login();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { statusCode: 401, body: { code: 16 } });
    openApplications();
    cy.location('pathname').should('eq', '/login');
  });
});

describe('JT-78 boundaries and recovery', () => {
  for (const [statusCode, code, text] of [[409, 6, 'subscription already exists'], [400, 9, 'at most 10 subscriptions']]) {
    it(`reports alert constraint ${code}`, () => {
      cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
      cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
      cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
      cy.intercept('POST', `${gateway}/v1/job-alerts`, { statusCode, body: { code } });
      login(); cy.contains('a', 'Job alerts').click();
      selectLocation('Thimphu'); cy.contains('button', 'Create subscription').click();
      cy.get('[role=alert]').should('contain', text);
    });
  }
  it('paginates reference data and submits Any as blank with category/tag filters', () => {
    const categoryId = '10000000-0000-4000-8000-000000000001';
    const tagId = '20000000-0000-4000-8000-000000000001';
    cy.intercept('GET', `${gateway}/v1/categories?*`, req => {
      req.reply(req.query['pagination.page_token'] === '20'
        ? { categories: [{ categoryId, name: 'Engineering' }] }
        : { categories: [], pagination: { nextPageToken: '20' } });
    });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId, name: 'Go' }] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [categoryId], tagIds: [tagId], location: '' }); req.reply({ jobAlert: {} });
    }).as('filters');
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').first().type('Engineering'); cy.get('[role=option]').contains('Engineering').click();
    cy.get('[role=combobox]').eq(1).type('Go'); cy.get('[role=option]').contains('Go').click();
    selectLocation('Thimphu');
    selectLocation('Any');
    cy.contains('button', 'Create subscription').click(); cy.wait('@filters');
  });
  for (const path of ['/applicant/apply', '/applicant/offers', '/applicant/job-alerts', `/employer/offers/new/${applicationId}`, `/employer/messages/${applicationId}`,
    '/employer/applicants', `/employer/applicants/${applicationId}`]) {
    it(`protects direct unauthenticated route ${path}`, () => {
      cy.visit(path); cy.location('pathname').should('eq', '/login');
    });
  }
  it('does not expose applicant or employer workflows to admins', () => {
    login('ROLE_ADMIN'); navigate('/applicant/applications'); cy.location('pathname').should('eq', '/admin');
    navigate(`/employer/offers/new/${applicationId}`); cy.location('pathname').should('eq', '/admin');
    navigate(`/employer/applicants/${applicationId}`); cy.location('pathname').should('eq', '/admin');
  });
});


describe('JT-78 location reference selection', () => {
  it('waits for location references before opening the non-native select', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    let releaseLocations;
    const referencesReady = new Promise(resolve => { releaseLocations = resolve; });
    cy.intercept('GET', `${gateway}/v1/locations?*`, req => referencesReady.then(() => {
      req.reply({ locations: [{ locationId: 'slow', name: 'Thimphu' }] });
    })).as('slowLocations');
    login(); cy.contains('a', 'Job alerts').click();
    // An early click is ignored, even though Cypress permits it on this div.
    locationSelect().should('have.attr', 'aria-disabled', 'true').click();
    locationSelect().should('have.attr', 'aria-expanded', 'false');
    cy.get('[role="listbox"]').should('not.exist');
    cy.then(() => releaseLocations());
    selectLocation('Thimphu');
    cy.wait('@slowLocations');
    locationSelect().should('contain', 'Thimphu');
    cy.contains('button', 'Create subscription').should('be.enabled');
  });
  it('loads every location reference page and submits the exact returned name', () => {
    const name = 'Remote / Hybrid';
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('GET', `${gateway}/v1/locations?*`, req => {
      expect(req.headers.authorization).to.eq('Bearer workflow-token');
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { locations: [{ locationId: 'later', name }] }
        : { locations: [{ locationId: 'first', name: 'Thimphu' }], pagination: { nextPageToken: '20' } });
    }).as('locations');
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [], tagIds: [], location: name });
      req.reply({ jobAlert: {} });
    }).as('exactLocation');
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').last().should('not.have.attr', 'aria-disabled', 'true').click();
    cy.get('[role=option]').should('have.length', 3);
    cy.get('[role=option]').contains(name).click();
    cy.contains('button', 'Create subscription').click(); cy.wait('@exactLocation');
    cy.get('@locations.all').should('have.length', 2);
  });
  it('preserves reference loading/error states and supports refresh after location failure', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('GET', `${gateway}/v1/locations?*`, { delay: 300, statusCode: 503, body: {} });
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').last().should('have.attr', 'aria-disabled', 'true');
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: 'restored', name: 'Thimphu' }] });
    cy.contains('button', 'Refresh subscriptions').click();
    selectLocation('Thimphu');
    cy.contains('button', 'Create subscription').should('be.enabled');
  });
});
