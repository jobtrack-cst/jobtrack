import { openAccountMenu } from './account-menu';
const gateway = 'http://localhost:8000';
const key = 'jobtrack.auth';
beforeEach(() => cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} }));
const user = role => ({ userId: '11111111-1111-4111-8111-111111111111', email: 'pema@example.test', fullName: 'Pema', role });
const credentials = (offset = 3600000) => ({ accessToken: 'session-token', expiresAt: Date.now() + offset });
function visitStored(value, path = '/applicant') {
  cy.visit(path, { onBeforeLoad(win) { win.sessionStorage.setItem(key, typeof value === 'string' ? value : JSON.stringify(value)); } });
}
function emptyStorage() {
  cy.window().should(win => expect(win.sessionStorage.getItem(key)).to.equal(null));
}
function submit() {
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
}
function client() {
  return cy.window().then(win => win.eval('import("/src/api/client.ts")'));
}

describe('JT-106 session persistence', () => {
  for (const [role, base] of [['ROLE_APPLICANT', '/applicant'], ['ROLE_EMPLOYER', '/employer'], ['ROLE_ADMIN', '/admin']]) {
    it(`confirms ${role} login, reload and direct deep links without extending expiry`, () => {
      const saved = credentials();
      cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: saved.accessToken, expiresAt: new Date(saved.expiresAt).toISOString(), user: user('ROLE_ADMIN') });
      cy.intercept('GET', `${gateway}/v1/auth/me`, req => {
        expect(req.headers.authorization).to.equal(`Bearer ${saved.accessToken}`);
        req.reply({ user: user(role) });
      }).as('me');
      cy.visit('/login');
      submit();
      cy.wait('@me');
      cy.location('pathname').should('eq', base);
      cy.window().should(win => {
        expect(JSON.parse(win.sessionStorage.getItem(key))).to.deep.equal(saved);
        expect(win.localStorage.length).to.equal(0);
      });
      cy.reload();
      cy.wait('@me');
      cy.location('pathname').should('eq', base);
      // Report form has no background reads and is available to every role.
      cy.visit(`${base}/report-job?from=reload`);
      cy.wait('@me');
      cy.location('pathname').should('eq', `${base}/report-job`);
      cy.location('search').should('eq', '?from=reload');
      cy.window().should(win => expect(JSON.parse(win.sessionStorage.getItem(key))).to.deep.equal(saved));
      openAccountMenu();
      cy.contains('button', 'Log out').click();
      emptyStorage();
      cy.reload();
      cy.location('pathname').should('eq', '/login');
    });
  }
  it('holds deep-link routing and protected content until revalidation finishes', () => {
    let release;
    cy.intercept('GET', `${gateway}/v1/auth/me`, req => new Promise(resolve => {
      release = () => { req.reply({ user: user('ROLE_EMPLOYER') }); resolve(); };
    })).as('me');
    visitStored(credentials(), '/employer/report-job');
    cy.contains('[role=status]', 'Restoring session').should('be.visible');
    cy.location('pathname').should('eq', '/employer/report-job');
    cy.get('input[name=email]').should('not.exist');
    cy.get('nav').should('not.exist');
    cy.then(() => release());
    cy.wait('@me');
    cy.get('nav').should('be.visible');
    cy.location('pathname').should('eq', '/employer/report-job');
  });
  for (const [name, value] of [
    ['invalid JSON', '{'], ['null', 'null'], ['array', '[]'], ['missing token', { expiresAt: Date.now() + 3600000 }],
    ['empty token', { accessToken: '', expiresAt: Date.now() + 3600000 }],
    ['malformed token', { accessToken: 'bad\ntoken', expiresAt: Date.now() + 3600000 }],
    ['wrong expiry type', { accessToken: 'token', expiresAt: 'tomorrow' }],
    ['expired', credentials(-1)], ['excessive lifetime', credentials(7200000)],
    ['identity only', { user: user('ROLE_ADMIN') }],
  ]) {
    it(`discards ${name} storage before making authenticated requests`, () => {
      const me = cy.stub().as('me');
      cy.intercept('GET', `${gateway}/v1/auth/me`, me);
      visitStored(value);
      cy.location('pathname').should('eq', '/login');
      emptyStorage();
      cy.get('@me').should('not.have.been.called');
    });
  }
  for (const status of [401, 403, 500]) {
    it(`discards storage when restore returns ${status}`, () => {
      cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: status, body: {} }).as('me');
      visitStored(credentials());
      cy.wait('@me');
      cy.location('pathname').should('eq', '/login');
      emptyStorage();
      client().then(module => {
        cy.intercept('GET', `${gateway}/v1/notifications`, req => {
          expect(req.headers).not.to.have.property('authorization');
          req.reply({ body: {} });
        });
        // Queue the API call after the interceptor is installed.
        cy.then(() => module.api('/v1/notifications'));
      });
    });
  }
  it('discards a restore with an unsupported identity', () => {
    cy.intercept('GET', `${gateway}/v1/auth/me`, { user: user('ROLE_UNSPECIFIED') });
    visitStored({ ...credentials(), user: user('ROLE_ADMIN') });
    cy.location('pathname').should('eq', '/login');
    emptyStorage();
  });
  it('clears storage and client auth on automatic one-hour expiry', () => {
    cy.intercept('GET', `${gateway}/v1/auth/me`, { user: user('ROLE_EMPLOYER') });
    visitStored(credentials(), '/employer');
    cy.contains('h1', /^Home$/).should('be.visible');
    cy.clock(Date.now(), ['Date', 'setTimeout', 'clearTimeout']);
    // The existing expiry timer predates cy.clock; focus also checks wall-clock expiry.
    cy.tick(3600001);
    cy.window().trigger('focus');
    cy.location('pathname').should('eq', '/login');
    emptyStorage();
  });
  it('clears a confirmed session on protected 401', () => {
    cy.intercept('GET', `${gateway}/v1/auth/me`, { user: user('ROLE_EMPLOYER') });
    visitStored(credentials(), '/employer');
    openAccountMenu();
    cy.contains('button', 'Log out').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: 401, body: {} });
    client().then(module => module.api('/v1/auth/me').catch(error => expect(error.status).to.equal(401)));
    cy.location('pathname').should('eq', '/login');
    emptyStorage();
  });
  it('allows in-memory login and logout when browser storage is disabled', () => {
    cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'token', expiresAt: new Date(Date.now() + 3600000).toISOString() });
    cy.intercept('GET', `${gateway}/v1/auth/me`, { user: user('ROLE_EMPLOYER') });
    cy.visit('/login', { onBeforeLoad(win) {
      Object.defineProperty(win, 'sessionStorage', { get() { throw new Error('Storage disabled'); } });
    } });
    submit();
    cy.location('pathname').should('eq', '/employer');
    openAccountMenu();
    cy.contains('button', 'Log out').click();
    cy.location('pathname').should('eq', '/login');
  });
  it('does not persist a login before /me confirms it', () => {
    cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'token', expiresAt: new Date(Date.now() + 3600000).toISOString() });
    cy.intercept('GET', `${gateway}/v1/auth/me`, { statusCode: 401, body: {} }).as('me');
    cy.visit('/login');
    submit();
    cy.wait('@me');
    cy.get('[role=alert]').should('contain', 'Invalid credentials');
    emptyStorage();
  });
  it('does not expose a restored token that expires while /me is pending', () => {
    let release;
    cy.intercept('GET', `${gateway}/v1/auth/me`, req => new Promise(resolve => {
      release = () => { req.reply({ user: user('ROLE_ADMIN') }); resolve(); };
    })).as('me');
    visitStored(credentials(), '/admin');
    cy.contains('[role=status]', 'Restoring session').should('be.visible');
    cy.wrap(null).should(() => expect(release).to.be.a('function'));
    cy.clock(Date.now(), ['Date']);
    cy.tick(3600001);
    cy.then(() => release());
    cy.wait('@me');
    cy.location('pathname').should('eq', '/login');
    emptyStorage();
  });
});

// Mount the real provider with a hook probe to exercise concurrent actions that
// the UI deliberately disables (double login and logout while /me is pending).
function provider() {
  cy.visit('/login');
  cy.window().then(async win => {
    // Use the app's exact Vite module URLs, including its dependency cache version.
    // Unversioned imports can create a second React dispatcher after chunk changes.
    const loadedModule = file => {
      const entry = win.performance.getEntriesByType('resource').find(resource =>
        new URL(resource.name).pathname === `/node_modules/.vite/deps/${file}.js`);
      expect(entry, `loaded ${file}`).to.exist;
      return win.eval(`import(${JSON.stringify(entry.name)})`);
    };
    const { default: React } = await loadedModule('react');
    const { default: { createRoot } } = await loadedModule('react-dom_client');
    const { AuthProvider, useAuth } = await win.eval('import("/src/features/auth/AuthProvider.tsx")');
    const host = win.document.createElement('div');
    win.document.body.appendChild(host);
    const root = createRoot(host);
    function Probe() { win.testAuth = useAuth(); return null; }
    root.render(React.createElement(AuthProvider, null, React.createElement(Probe)));
  });
  cy.window().should(win => expect(win.testAuth?.initializing).to.equal(false));
}

describe('JT-106 async session races', () => {
  for (const pending of ['login', 'me']) {
    it(`ignores a late ${pending} success after logout`, () => {
      provider();
      let release;
      cy.window().then(win => {
        cy.stub(win, 'fetch').callsFake(url => {
          const body = url.endsWith('/login') ? { accessToken: 'token', expiresAt: new Date(Date.now() + 3600000).toISOString() } : { user: user('ROLE_EMPLOYER') };
          if (url.endsWith(`/${pending}`)) return new Promise(resolve => { release = () => resolve(new win.Response(JSON.stringify(body))); });
          return Promise.resolve(new win.Response(JSON.stringify(body)));
        });
        win.pendingLogin = win.testAuth.signIn('pema@example.test', 'password123').catch(() => {});
      });
      cy.wrap(null).should(() => expect(release).to.be.a('function'));
      cy.window().then(win => { win.testAuth.signOut(); release(); return win.pendingLogin; });
      cy.window().should(win => expect(win.testAuth.session).to.equal(null));
      emptyStorage();
    });
  }
  for (const status of [200, 401, 500]) {
    it(`ignores an older /me HTTP ${status} after a replacement login, even with the same token`, () => {
      provider();
      let release;
      let calls = 0;
      cy.window().then(win => {
        cy.stub(win, 'fetch').callsFake(url => {
          if (url.endsWith('/login')) return Promise.resolve(new win.Response(JSON.stringify({ accessToken: 'same-token', expiresAt: new Date(Date.now() + 3600000).toISOString() })));
          if (++calls === 1) return new Promise(resolve => { release = () => resolve(new win.Response(JSON.stringify({ user: user('ROLE_EMPLOYER') }), { status })); });
          return Promise.resolve(new win.Response(JSON.stringify({ user: user('ROLE_ADMIN') })));
        });
        win.pendingLogin = win.testAuth.signIn('old@example.test', 'password123').catch(() => {});
      });
      cy.wrap(null).should(() => expect(release).to.be.a('function'));
      cy.window().then(win => win.testAuth.signIn('new@example.test', 'password123'));
      cy.window().should(win => expect(win.testAuth.session?.user.role).to.equal('ROLE_ADMIN'));
      cy.window().then(win => { release(); return win.pendingLogin; });
      cy.window().should(win => expect(win.testAuth.session?.user.role).to.equal('ROLE_ADMIN'));
      cy.window().should(win => expect(JSON.parse(win.sessionStorage.getItem(key)).accessToken).to.equal('same-token'));
    });
  }
});
