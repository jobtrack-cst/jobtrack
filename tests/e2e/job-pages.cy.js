import { settleHome } from './home';
import { openAccountMenu } from './account-menu';
// JT-73 gateway-stubbed notifications and job-route checks. The employer job pages were
// rebuilt in JT-111 and their rules moved to employer-pages.cy.js ("JT-111 jobs (JT-73 rules)");
// the applicant job pages were rebuilt in JT-112 (applicant-pages.cy.js).
const gateway = 'http://localhost:8000';
// Keep the applicant app-bar photo request independent of a running gateway.
beforeEach(() => cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} }));
// JT-102 job cards read company profiles; keep them independent of a running gateway.
beforeEach(() => cy.intercept('GET', `${gateway}/v1/companies/**`, { statusCode: 404, body: { code: 5 } }));
const notificationId = '55555555-5555-4555-8555-555555555555';
const userId = '11111111-1111-4111-8111-111111111111';
const notice = { notificationId, title: 'Interview scheduled', body: '<script>private script</script>', createdAt: new Date().toISOString() };
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
const card = text => cy.contains('.MuiCard-root', text);
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
function login(role = 'ROLE_EMPLOYER') {
  const user = { userId, fullName: 'Pema', email: 'pema@example.test', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt73-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt73-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', `/${role.slice(5).toLowerCase()}`);
  settleHome();
}

describe('JT-73 notifications and routing', () => {
  for (const role of ['ROLE_APPLICANT', 'ROLE_EMPLOYER', 'ROLE_ADMIN']) {
    it(`lists and marks notifications read for ${role}`, () => {
      let read = false;
      cy.intercept('GET', `${gateway}/v1/notifications?*`, req => {
        req.reply({ notifications: req.query.unread_only === 'true' && read ? [] : [{ ...notice, read }], pagination: { totalCount: req.query.unread_only === 'true' && read ? 0 : 1 } });
      }).as('notifications');
      cy.intercept('POST', `${gateway}/v1/notifications/${notificationId}/read`, req => {
        expect(req.body).to.deep.eq({}); read = true; req.reply({ notification: { ...notice, read } });
      }).as('read');
      login(role); visit(`/${role.slice(5).toLowerCase()}/notifications`); cy.wait('@notifications');
      card(notice.title).should('contain.text', notice.body).find('script').should('not.exist');
      cy.contains('label', 'Unread only').click(); cy.wait('@notifications').its('request.query.unread_only').should('eq', 'true');
      card(notice.title).contains('button', 'Mark as read').click(); cy.wait('@read');
      cy.contains('No notifications on this page.').should('be.visible'); cy.contains('Unread notifications: 0').should('be.visible');
      cy.contains('label', 'Unread only').click(); card(notice.title).should('contain', 'Read');
      cy.contains('button', 'Mark as read').should('not.exist');
    });
  }

  it('paginates notifications and handles mark-read failures safely', () => {
    cy.intercept('GET', `${gateway}/v1/notifications?*`, req => req.reply({ notifications: [notice], pagination: req.query['pagination.page_token'] ? {} : { nextPageToken: '20' } })).as('list');
    login('ROLE_APPLICANT'); visit('/applicant/notifications'); cy.wait('@list');
    cy.contains('button', 'Next page').click(); cy.wait('@list').its('request.query').should('include', { 'pagination.page_token': '20' });
    cy.intercept('POST', `${gateway}/v1/notifications/${notificationId}/read`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    cy.contains('button', 'Mark as read').click(); cy.get('[role=alert]').should('contain', 'own notifications').and('not.contain', 'private-input');
    cy.contains('button', 'Mark as read').should('be.enabled');
  });

  it('clears an expired session on a protected notification request', () => {
    cy.intercept('GET', `${gateway}/v1/notifications?*`, { statusCode: 401, body: { code: 16 } });
    login('ROLE_APPLICANT'); visit('/applicant/notifications'); cy.location('pathname').should('eq', '/login');
  });

  it('restricts job routes and navigation to the correct role', () => {
    login('ROLE_APPLICANT'); cy.contains('nav a', exact('Jobs')).should('not.exist');
    visit('/employer/jobs'); cy.location('pathname').should('eq', '/applicant');
    openAccountMenu();
    cy.contains('button', 'Log out').click(); login();
    cy.contains('nav a', exact('Find jobs')).should('not.exist'); cy.contains('nav a', exact('Saved')).should('not.exist');
    visit('/applicant/saved'); cy.location('pathname').should('eq', '/employer');
    openAccountMenu();
    cy.contains('button', 'Log out').click(); login('ROLE_ADMIN');
    visit('/employer/jobs/new'); cy.location('pathname').should('eq', '/admin');
  });

  it('requires login for the new protected pages', () => {
    for (const path of ['/employer/jobs/new', '/applicant/jobs', '/applicant/saved', '/admin/notifications']) {
      cy.visit(path); cy.location('pathname').should('eq', '/login');
    }
  });
});
