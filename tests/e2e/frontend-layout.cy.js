// JT-101: presentation regressions beyond the representative report captures.
import { openNavigation, closeNavigation } from './navigation';
import { homeHeading, navigationLinks as links } from './home';
const gateway = 'http://localhost:8000';

for (const role of Object.keys(links)) {
  it(`JT-101 ${role}: all navigation pages fit mobile and tablet, with keyboard access to main content`, () => {
    const user = { userId: '11111111-1111-4111-8111-111111111111', fullName: 'Synthetic User', email: 'synthetic@example.test', role: `ROLE_${role.toUpperCase()}` };
    cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { userId: user.userId, summary: '', education: [], skills: [] } });
    cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} });
    cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile: { employerId: user.userId, companyName: 'Synthetic Company', verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED' } });
    cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt101-layout', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
    cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
    cy.visit('/login');
    cy.get('input[name=email]').type(user.email);
    cy.get('input[name=password]').type('synthetic-password', { log: false });
    cy.get('button[type=submit]').click();
    cy.location('pathname').should('eq', `/${role}`);
    cy.get('.workspace-content h1').should('be.visible');
    for (const width of [320, 390, 768, 1024, 1280]) {
      cy.viewport(width, 900);
      for (const label of links[role]) {
        openNavigation();
        cy.contains('nav a', new RegExp(`^${label}$`)).click();
        openNavigation();
        cy.contains('nav a', new RegExp(`^${label}$`)).should('have.attr', 'aria-current', 'page');
        closeNavigation();
        cy.get('.workspace-content h1').should('be.visible');
        cy.get('.workspace-content .MuiCircularProgress-root').should('not.exist');
        cy.document().then(doc => expect(doc.documentElement.scrollWidth, `${label} at ${width}px`).to.be.at.most(doc.documentElement.clientWidth));
      }
    }
    cy.contains('a', 'Skip to content').focus().should('be.visible');
    cy.press(Cypress.Keyboard.Keys.ENTER);
    cy.get('main').should('be.focused');
    cy.get('header a[aria-label="JobTrack home"]').click();
    cy.location('pathname').should('eq', `/${role}`);
    cy.get('main h1').should('have.text', homeHeading(role, user.fullName));
  });
}

it('JT-101 honors the browser reduced-motion preference', () => {
  cy.visit('/login');
  cy.then(() => Cypress.automation('remote:debugger:protocol', {
    command: 'Emulation.setEmulatedMedia', params: { features: [{ name: 'prefers-reduced-motion', value: 'reduce' }] },
  }));
  cy.window().then(win => expect(win.matchMedia('(prefers-reduced-motion: reduce)').matches).to.eq(true));
  cy.get('button[type=submit]').should($button => {
    expect($button[0].ownerDocument.defaultView.getComputedStyle($button[0]).transitionDuration).to.eq('0s');
  });
  cy.then(() => Cypress.automation('remote:debugger:protocol', {
    command: 'Emulation.setEmulatedMedia', params: { features: [] },
  }));
});

it('JT-110 brand keeps its wordmark and accessible name when the mark fails to load', () => {
  cy.intercept('GET', '/branding/jobtrack-mark.svg', { statusCode: 404, body: '' });
  cy.visit('/register');
  cy.get('header a[aria-label="JobTrack home"]').should('contain', 'JobTrack');
  cy.get('header a[aria-label="JobTrack home"] img').should('have.attr', 'alt', '');
  cy.get('header a[aria-label="JobTrack home"]').focus().should('be.focused');
  cy.press(Cypress.Keyboard.Keys.ENTER);
  cy.location('pathname').should('eq', '/login');
  cy.get('main h1').should('have.text', 'Log in');
});

it('JT-101 long job data reflows at all five widths and at 200% text size', () => {
  const user = { userId: '11111111-1111-4111-8111-111111111111', fullName: 'Synthetic Applicant', email: 'synthetic@example.test', role: 'ROLE_APPLICANT' };
  cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt101-long-data', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [{ jobId: '22222222-2222-4222-8222-222222222222', title: '界'.repeat(150), location: '長'.repeat(100), deadline: '2030-12-20T12:00:00Z' }], pagination: { totalCount: 1 } });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('synthetic-password', { log: false });
  cy.get('button[type=submit]').click();
  openNavigation();
  cy.contains('nav a', /^Find jobs$/).click();
  cy.get('.MuiCard-root h2').should('contain', '界'.repeat(150));
  for (const width of [320, 390, 768, 1024, 1280]) {
    cy.viewport(width, 900);
    cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
    cy.get('.MuiCard-root').each($card => expect($card[0].scrollWidth).to.be.at.most($card[0].clientWidth));
  }
  cy.document().then(doc => { doc.documentElement.style.fontSize = '200%'; });
  cy.get('.MuiCard-root h2').should('be.visible');
  cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  cy.get('nav a').each($link => expect($link[0].scrollWidth).to.be.at.most($link[0].clientWidth));
  cy.document().then(doc => { doc.documentElement.style.fontSize = ''; });
});
