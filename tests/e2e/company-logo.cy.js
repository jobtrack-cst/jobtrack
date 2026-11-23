// JT-102 company logo on job cards and job details, gateway-stubbed.
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
const withLogo = '66666666-6666-4666-8666-666666666666';
const noLogo = '77777777-7777-4777-8777-777777777777';
const deadline = new Date(Date.now() + 7 * 86400000).toISOString();
const job = (jobId, employerId, title) => ({ jobId, employerId, title, location: 'Thimphu', deadline });
const jobs = [
  job('22222222-2222-4222-8222-222222222222', withLogo, 'Frontend intern'),
  job('33333333-3333-4333-8333-333333333333', withLogo, 'Backend intern'),
  job('88888888-8888-4888-8888-888888888888', noLogo, 'Data intern'),
];
// A real 1x1 PNG, so the browser can draw it and MUI keeps the img.
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAIAAACQd1PeAAAADElEQVR4nGNQSQsHAAGTAOLTSNOPAAAAAElFTkSuQmCC';
// JT-112 job cards are list items; the logo is decorative because the company name is shown as text.
const card = text => cy.contains('ul[aria-label=Jobs] > li', text);
const logo = $el => cy.wrap($el).find('.MuiAvatar-root img');

beforeEach(() => {
  // Defined first, so it only answers calls no other stub handles; never reaches a real gateway.
  cy.intercept(`${gateway}/v1/**`, { statusCode: 404, body: { code: 5 } });
  const user = { userId, fullName: 'Pema', email: 'pema@example.test', role: 'ROLE_APPLICANT' };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt102-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs, pagination: { totalCount: jobs.length } });
  cy.intercept('GET', `${gateway}/v1/companies/${withLogo}`, { profile: { employerId: withLogo, companyName: 'Druk Tech', hasLogo: true } }).as('profile');
  cy.intercept('GET', `${gateway}/v1/companies/${withLogo}/logo`, { contentType: 'image/png', content: png }).as('logo');
  // protojson omits has_logo when false.
  cy.intercept('GET', `${gateway}/v1/companies/${noLogo}`, { profile: { employerId: noLogo, companyName: 'Paro Works' } });
  cy.intercept('GET', `${gateway}/v1/companies/${noLogo}/logo`, { statusCode: 404, body: { code: 5 } }).as('noLogo');
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', '/applicant');
});

describe('JT-102 company logo on jobs', () => {
  it('shows the logo on search cards and loads it once per employer', () => {
    cy.contains('nav a', /^Find jobs$/).click();
    for (const title of ['Frontend intern', 'Backend intern']) {
      card(title).should('contain', 'Druk Tech').then(logo)
        .should('have.attr', 'src').and('match', /^data:image\/png;base64,/);
    }
    cy.get('@profile.all').should('have.length', 1);
    cy.get('@logo.all').should('have.length', 1);
  });

  it('shows initials and skips the logo request when the company has no logo', () => {
    cy.contains('nav a', /^Find jobs$/).click();
    card('Data intern').should('contain', 'Paro Works').find('.MuiAvatar-root').should('have.text', 'PW').find('img').should('not.exist');
    cy.get('@noLogo.all').should('have.length', 0);
  });

  it('shows the logo on job details', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobs[0].jobId}`, { job: { ...jobs[0], description: 'Build pages', status: 'JOB_STATUS_PUBLISHED' } });
    cy.contains('nav a', /^Find jobs$/).click();
    card('Frontend intern').contains('a', 'View job').click();
    cy.contains('Build pages').should('be.visible');
    cy.contains('main section', 'Build pages').should('contain', 'Druk Tech').then(logo)
      .should('have.attr', 'src').and('match', /^data:image\/png;base64,/);
  });
});
