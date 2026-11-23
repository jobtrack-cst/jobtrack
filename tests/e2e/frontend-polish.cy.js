// JT-101: real browser captures with synthetic gateway responses, not live services.
import { openNavigation, closeNavigation } from './navigation';
import { openAccountMenu } from './account-menu';
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
const jobId = '22222222-2222-4222-8222-222222222222';
const job = { jobId, employerId: userId, title: 'Frontend developer intern', location: 'Thimphu',
  description: 'Help build accessible recruitment experiences.', deadline: '2030-12-20T12:00:00Z', status: 'JOB_STATUS_PUBLISHED' };
const jobs = [job, { ...job, jobId: '33333333-3333-4333-8333-333333333333', title: 'Graduate software engineer', location: 'Paro' }];

function capture(name) {
  let width;
  let height;
  // Capture in one frame to avoid Electron's full-page stitching artifacts.
  cy.document().then(doc => {
    width = doc.defaultView.innerWidth;
    height = doc.defaultView.innerHeight;
    doc.activeElement?.blur();
    const style = doc.createElement('style');
    style.id = 'capture-sticky-fix';
    style.textContent = 'header, aside { position: static !important; }';
    doc.head.append(style);
  });
  cy.document().then(doc => {
    const contentBottom = doc.querySelector('main').getBoundingClientRect().bottom + doc.defaultView.scrollY;
    return cy.viewport(width, Math.max(height, Math.ceil(contentBottom)));
  });
  cy.screenshot(name, { capture: 'viewport', scale: true, overwrite: true });
  cy.then(() => cy.viewport(width, height));
  cy.document().then(doc => doc.getElementById('capture-sticky-fix').remove());
}

function login(role) {
  const user = { userId, fullName: 'Pema Wangchuk', email: 'pema@example.test', role: `ROLE_${role.toUpperCase()}` };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt101-synthetic-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} });
  for (const key of ['categories', 'tags', 'locations']) cy.intercept('GET', `${gateway}/v1/${key}?*`, { [key]: [], pagination: {} });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('synthetic-password', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', `/${role}`);
}

describe('JT-101 shared presentation', () => {
  for (const role of ['applicant', 'employer', 'admin']) {
    it(`${role}: captures all five widths, preserves role navigation and readable content`, () => {
      cy.viewport(1280, 900);
      // Registered first, so the specific stubs below take precedence.
      cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
      cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs, pagination: { totalCount: jobs.length } });
      cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, { jobs, pagination: { totalCount: jobs.length } });
      cy.intercept('GET', `${gateway}/v1/verification-requests*`, { requests: [], pagination: {} });
      login(role);
      capture(`${role}-home-desktop`);
      cy.viewport(390, 844);
      capture(`${role}-home-mobile`);
      openNavigation();
      cy.get('[role=dialog]').should('be.visible');
      cy.screenshot(`${role}-navigation-mobile`, { capture: 'viewport', overwrite: true });
      closeNavigation();
      cy.viewport(1280, 900);
      const page = role === 'applicant' ? 'Find jobs' : role === 'employer' ? 'Jobs' : 'Analytics';
      cy.contains('nav a', new RegExp(`^${page}$`)).click();
      if (role === 'admin') {
        cy.intercept('GET', `${gateway}/v1/analytics/summary*`, req => req.reply({ summary: { startTime: req.query.start_time, endTime: req.query.end_time, jobsPublished: 24, applicationsSubmitted: 86, offersMade: 12, offersAccepted: 9, statusChanges: [{ status: 'SHORTLISTED', count: 18 }] } }));
        cy.contains('button', 'Show summary').click();
        cy.contains('td', '86').should('be.visible');
      } else cy.contains(role === 'applicant' ? 'ul[aria-label=Jobs] > li' : '.MuiCard-root', job.title).should('be.visible');
      cy.get('main h1').should('have.length', 1).and('be.visible');
      capture(`${role}-desktop`);
      cy.viewport(1024, 900);
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      capture(`${role}-laptop`);
      cy.viewport(768, 1024);
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      capture(`${role}-tablet`);
      cy.viewport(390, 844);
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.get('main h1').should('be.visible');
      capture(`${role}-mobile`);
      cy.viewport(320, 760);
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      capture(`${role}-small`);
      cy.contains('nav a', role === 'applicant' ? /^Jobs$/ : /^Find jobs$/).should('not.exist');
      openAccountMenu();
      cy.screenshot(`${role}-account-small`, { capture: 'viewport', overwrite: true });
      cy.press(Cypress.Keyboard.Keys.ESC);
      cy.get('#account-menu').should('not.exist');
      cy.viewport(1280, 900);
      openAccountMenu();
      cy.screenshot(`${role}-account-desktop`, { capture: 'viewport', overwrite: true });
    });
  }

  it('login has keyboard reachable form controls without mobile overflow', () => {
    cy.viewport(390, 844);
    cy.visit('/login');
    cy.get('input[name=email]').focus().should('be.focused');
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.get('input[name=password]').should('be.focused');
    cy.press(Cypress.Keyboard.Keys.TAB);
    cy.get('button[type=submit]').should('be.focused');
    cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  });

  for (const registration of [false, true]) it(`captures ${registration ? 'registration' : 'login'} branding`, () => {
    cy.viewport(1280, 900);
    cy.visit(registration ? '/register' : '/login');
    cy.get('main h1').should('be.visible');
    capture(registration ? 'register-desktop' : 'login-desktop');
    cy.viewport(390, 844);
    cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
    capture(registration ? 'register-mobile' : 'login-mobile');
    cy.viewport(320, 760);
    cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
    capture(registration ? 'register-small' : 'login-small');
  });
});
