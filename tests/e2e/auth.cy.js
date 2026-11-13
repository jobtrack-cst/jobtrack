const gateway = 'http://localhost:8000';
const user = (role = 'ROLE_APPLICANT') => ({ userId: '11111111-1111-4111-8111-111111111111', email: 'pema@example.test', fullName: 'Pema', role });
function signIn(role = 'ROLE_APPLICANT', seconds = 3600) {
  cy.intercept('POST', `${gateway}/v1/auth/login`, (req) => {
    expect(req.body).to.deep.equal({ email: 'pema@example.test', password: 'password123' });
    expect(req.headers).not.to.have.property('authorization');
    req.reply({ accessToken: 'test-token', expiresAt: new Date(Date.now() + seconds * 1000).toISOString(), user: user(role) });
  }).as('login');
  cy.intercept('GET', `${gateway}/v1/auth/me`, (req) => {
    expect(req.headers.authorization).to.equal('Bearer test-token');
    req.reply({ user: user(role) });
  }).as('me');
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.wait(['@login', '@me']);
}

describe('JT-63 authentication and routing', () => {
  for (const path of ['/applicant', '/employer', '/admin']) {
    it(`redirects unauthenticated ${path} to login without looping`, () => {
      cy.visit(path);
      cy.location('pathname').should('eq', '/login');
      cy.contains('h1', 'Log in').should('be.visible');
    });
  }
  for (const [role, path, other] of [
    ['ROLE_APPLICANT', '/applicant', '/admin'],
    ['ROLE_EMPLOYER', '/employer', '/applicant'],
    ['ROLE_ADMIN', '/admin', '/employer'],
  ]) {
    it(`logs in ${role}, enforces its route and logs out`, () => {
      signIn(role);
      cy.location('pathname').should('eq', path);
      cy.window().then((win) => win.history.pushState({}, '', other));
      cy.window().trigger('popstate');
      cy.location('pathname').should('eq', path);
      cy.contains('button', 'Log out').click();
      cy.location('pathname').should('eq', '/login');
      cy.reload();
      cy.location('pathname').should('eq', '/login');
    });
  }
  it('does not persist the session across reloads', () => {
    signIn();
    cy.location('pathname').should('eq', '/applicant');
    cy.reload();
    cy.location('pathname').should('eq', '/login');
  });
  it('rejects an unsupported account role', () => {
    signIn('ROLE_UNSPECIFIED');
    cy.location('pathname').should('eq', '/login');
    cy.get('[role=alert]').should('contain', 'Unsupported account role');
  });
  it('keeps invalid login on login and permits retry', () => {
    cy.intercept('POST', `${gateway}/v1/auth/login`, { statusCode: 401, body: { code: 16, message: 'invalid email or password' } }).as('badLogin');
    cy.visit('/login');
    cy.get('input[name=email]').type('pema@example.test');
    cy.get('input[name=password]').type('wrong-password', { log: false });
    cy.get('button[type=submit]').click();
    cy.wait('@badLogin');
    cy.get('[role=alert]').should('contain', 'Invalid credentials');
    cy.location('pathname').should('eq', '/login');
    cy.get('button[type=submit]').should('be.enabled');
  });
  it('registers using the real contract, then requires login', () => {
    cy.intercept('POST', `${gateway}/v1/auth/register`, (req) => {
      expect(req.headers).not.to.have.property('authorization');
      expect(req.body).to.deep.equal({ email: 'pema@example.test', password: 'password123', fullName: 'Pema', role: 'ROLE_EMPLOYER' });
      req.reply({ user: user('ROLE_EMPLOYER') });
    }).as('register');
    cy.visit('/register');
    cy.get('input[name=fullName]').type('Pema');
    cy.get('input[name=email]').type('pema@example.test');
    cy.get('input[name=password]').type('password123', { log: false });
    cy.get('[role=combobox]').click();
    cy.get('[role=option]').should('have.length', 2);
    cy.get('[role=option]').contains('Employer').click();
    cy.get('button[type=submit]').click();
    cy.wait('@register');
    cy.location('pathname').should('eq', '/login');
    cy.contains('Account created').should('be.visible');
  });
  it('expires the session without attempting refresh', () => {
    signIn('ROLE_APPLICANT', 2);
    cy.location('pathname').should('eq', '/applicant');
    cy.location('pathname', { timeout: 5000 }).should('eq', '/login');
  });
});

describe('JT-63 shared API client', () => {
  // Load the real Vite module in the application window, not the Cypress runner.
  function client() {
    return cy.window().then((win) => win.eval('import("/src/api/client.ts")'));
  }
  beforeEach(() => cy.visit('/login'));
  it('omits cookies and uses only the direct gateway origin', () => {
    cy.window().then((win) => cy.stub(win, 'fetch').resolves(new win.Response('{}', { status: 200 })).as('fetch'));
    client().then(async (module) => {
      module.configureAuth('test-token', () => {});
      await module.api('/v1/auth/me');
    });
    cy.get('@fetch').should('have.been.calledWithMatch', `${gateway}/v1/auth/me`, { credentials: 'omit', headers: { Authorization: 'Bearer test-token' } });
  });
  it('handles protected 401 once and ignores public 401', () => {
    cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: 401, body: {} });
    client().then(async (module) => {
      let count = 0;
      module.configureAuth('test-token', () => { count++; module.configureAuth(null, () => {}); });
      await module.api('/v1/auth/me').catch((error) => expect(error.status).to.equal(401));
      expect(count).to.equal(1);
      await module.api('/v1/auth/me', { public: true }).catch(() => {});
      expect(count).to.equal(1);
    });
  });
  for (const status of [400, 403, 409, 500]) {
    it(`reports HTTP ${status} without expiring the session or exposing the body`, () => {
      cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: status, body: { message: 'sensitive-input' } });
      client().then(async (module) => {
        module.configureAuth('test-token', () => { throw new Error('Must not log out'); });
        await module.api('/v1/auth/me').then(() => { throw new Error('Expected rejection'); }, (error) => {
          expect(error.status).to.equal(status);
          expect(error.message).not.to.contain('sensitive-input');
        });
      });
    });
  }
  it('reports network failure and invalid JSON', () => {
    cy.intercept('GET', `${gateway}/v1/auth/me`, { forceNetworkError: true });
    client().then(async (module) => {
      await module.api('/v1/auth/me').catch((error) => expect(error.message).to.contain('Cannot reach'));
    });
    cy.intercept('GET', `${gateway}/v1/auth/me`, { body: 'not json' });
    client().then(async (module) => {
      await module.api('/v1/auth/me').catch((error) => expect(error.message).to.contain('invalid response'));
    });
  });
  it('redirects a logged in user when a protected request returns 401', () => {
    signIn();
    cy.location('pathname').should('eq', '/applicant');
    cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: 401, body: {} });
    client().then(async (module) => { await module.api('/v1/auth/me').catch(() => {}); });
    cy.location('pathname').should('eq', '/login');
  });
});
