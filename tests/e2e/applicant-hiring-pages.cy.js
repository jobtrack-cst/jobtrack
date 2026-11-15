// JT-83 pages. Gateway responses are stubbed, so these tests check the UI and
// the requests it sends, not live services (see docs/testing/jt-83-test-cases.md).
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
const year = new Date().getFullYear();

function login(role = 'ROLE_APPLICANT') {
  const user = { userId, email: 'pema@example.test', fullName: 'Pema', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt83-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt83-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', role === 'ROLE_APPLICANT' ? '/applicant' : '/employer');
}
function field(label) {
  return cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
}
// Long text is set with the native value setter and one input event, as in the
// JT-78 offer test: typing it key by key trips React's nested update limit
// inside Cypress, which does not happen in a real browser.
function fill(label, text) {
  field(label).then($el => {
    const el = $el[0];
    const view = el.ownerDocument.defaultView;
    const proto = el.tagName === 'TEXTAREA' ? view.HTMLTextAreaElement.prototype : view.HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, text);
    el.dispatchEvent(new view.Event('input', { bubbles: true }));
  });
}

describe('JT-83 applicant profile', () => {
  const profile = {
    applicantId: userId, summary: 'Synthetic summary',
    education: [{ institution: 'Synthetic College', qualification: 'BSc', startYear: 2020, endYear: 2024 }],
    skills: ['Go', 'React'],
  };

  it('creates a profile when the applicant has none', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { statusCode: 404, body: { code: 5, message: 'private' } }).as('load');
    login();
    cy.contains('a', 'Profile').click();
    cy.wait('@load');
    cy.contains('You do not have a profile yet').should('be.visible');
    field('Summary').type('Synthetic summary');
    cy.contains('button', 'Add education').click();
    field('Institution').type('Synthetic College');
    field('Qualification').type('BSc');
    field('Start year').type('2020');
    field('End year').type('2024');
    field('Skills').type('Go{enter}React{enter}');
    cy.intercept('POST', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body).to.deep.eq({ summary: profile.summary, education: profile.education, skills: profile.skills });
      req.reply({ profile });
    }).as('create');
    cy.contains('button', 'Create profile').click();
    cy.wait('@create');
    cy.contains('Profile created.').should('be.visible');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('updates an existing profile with PUT, sending an ongoing entry as end year 0', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile }).as('load');
    login();
    cy.contains('a', 'Profile').click();
    cy.wait('@load');
    field('Summary').should('have.value', 'Synthetic summary');
    field('End year').clear();
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body.education).to.deep.eq([{ ...profile.education[0], endYear: 0 }]);
      expect(req.body.skills).to.deep.eq(['Go', 'React']);
      req.reply({ profile: { ...profile, education: req.body.education } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
    cy.contains('Profile updated.').should('be.visible');
    field('End year').should('have.value', '');
  });

  it('removes an education entry and clears the list', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('button', 'Remove entry 1').click();
    cy.contains('No education entries.').should('be.visible');
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body.education).to.deep.eq([]);
      req.reply({ profile: { ...profile, education: [] } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
  });

  for (const [name, change, message] of [
    ['start year below 1950', () => { field('Start year').clear().type('1949'); }, 'start year must be from 1950'],
    ['start year after next year', () => { field('Start year').clear().type(String(year + 2)); }, 'start year must be from 1950'],
    ['end year before start year', () => { field('End year').clear().type('2019'); }, 'end year must be empty'],
    ['end year more than 10 years ahead', () => { field('End year').clear().type(String(year + 11)); }, 'end year must be empty'],
    ['empty institution', () => { field('Institution').clear(); }, 'institution needs 1 to 150'],
    ['institution over 150 characters', () => { fill('Institution', 'x'.repeat(151)); }, 'institution needs 1 to 150'],
    ['duplicate skill ignoring case', () => { field('Skills').type('go{enter}'); }, 'listed twice'],
  ]) {
    it(`blocks saving with ${name}`, () => {
      cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
      login();
      cy.contains('a', 'Profile').click();
      field('Summary').should('have.value', 'Synthetic summary');
      change();
      cy.get('[role=alert]').should('contain', message);
      cy.contains('button', 'Save profile').should('be.disabled');
    });
  }

  it('accepts boundary values: 150 character institution, start year next year, end year 10 years ahead', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    fill('Institution', 'x'.repeat(150));
    field('Institution').should('have.value', 'x'.repeat(150));
    field('Start year').clear().type(String(year + 1));
    field('End year').clear().type(String(year + 10));
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a fixed message for a rejected save and never the server text', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, { statusCode: 400, body: { code: 3, message: 'private-input' } });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('button', 'Save profile').click();
    cy.get('[role=alert]').should('contain', 'The profile was not saved').and('not.contain', 'private-input');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a load failure and retries', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { statusCode: 503, body: {} });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    cy.contains('button', 'Try again').click();
    field('Summary').should('have.value', 'Synthetic summary');
  });

  it('does not show the profile link to employers', () => {
    login('ROLE_EMPLOYER');
    cy.contains('a', 'Profile').should('not.exist');
  });
});
