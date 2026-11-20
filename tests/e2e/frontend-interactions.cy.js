const gateway = 'http://localhost:8000';

function login(role = 'applicant') {
  const user = { userId: '11111111-1111-4111-8111-111111111111', fullName: 'Synthetic User', email: 'synthetic@example.test', role: `ROLE_${role.toUpperCase()}` };
  cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
  cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { userId: user.userId, summary: '', education: [], skills: [] } });
  cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile: { employerId: user.userId, companyName: 'Synthetic Company', verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED' } });
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt101-interactions', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('synthetic-password', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', `/${role}`);
}

// Native Chromium input, including the Cypress iframe's rendered scale.
function inputAt($element, command, params) {
  cy.window().then(win => {
    const frame = (win.frameElement || window.top.document.querySelector('iframe.aut-iframe')).getBoundingClientRect();
    const scale = frame.width / win.innerWidth;
    const rect = $element[0].getBoundingClientRect();
    const x = frame.left + (rect.left + rect.width / 2) * scale;
    const y = frame.top + (rect.top + rect.height / 2) * scale;
    return Cypress.automation('remote:debugger:protocol', { command, params: command === 'Input.dispatchTouchEvent'
      ? { ...params, touchPoints: params.type === 'touchEnd' ? [] : [{ x, y }] }
      : { ...params, x, y } });
  });
}

function enterOnMenu() {
  cy.window().then(win => {
    win.focus();
    win.document.querySelector('button[aria-label="Open navigation"]').focus();
  });
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchKeyEvent',
    params: { type: 'keyDown', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13, text: '\r', unmodifiedText: '\r' } }));
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchKeyEvent',
    params: { type: 'keyUp', key: 'Enter', code: 'Enter', windowsVirtualKeyCode: 13 } }));
}

it('JT-101 native pointer hover, pressed and keyboard focus states are visible', () => {
  cy.viewport(1024, 900);
  cy.visit('/login');
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchMouseEvent', params: { type: 'mouseMoved', x: 0, y: 0 } }));
  let restingColor;
  cy.get('button[type=submit]').then($button => {
    restingColor = $button[0].ownerDocument.defaultView.getComputedStyle($button[0]).backgroundColor;
    inputAt($button, 'Input.dispatchMouseEvent', { type: 'mouseMoved' });
  });
  cy.get('button[type=submit]').should($button => {
    expect($button[0].matches(':hover')).to.eq(true);
    expect($button[0].ownerDocument.defaultView.getComputedStyle($button[0]).backgroundColor).not.to.eq(restingColor);
  }).then($button => inputAt($button, 'Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1 }));
  cy.get('button[type=submit]').should($button => {
    expect($button[0].matches(':active')).to.eq(true);
    expect($button[0].ownerDocument.defaultView.getComputedStyle($button[0]).transform).to.eq('matrix(1, 0, 0, 1, 0, 1)');
  });
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchMouseEvent', params: { type: 'mouseReleased', x: 0, y: 0, button: 'left', clickCount: 1 } }));
  cy.get('input[name=password]').focus();
  cy.press(Cypress.Keyboard.Keys.TAB);
  cy.get('button[type=submit]').should('be.focused').should($button => {
    const style = $button[0].ownerDocument.defaultView.getComputedStyle($button[0]);
    expect(style.outlineStyle).to.eq('solid');
    expect(style.outlineWidth).to.eq('3px');
  });
});

it('JT-101 mobile drawer supports touch, focus trapping, Escape and focus return', () => {
  cy.viewport(320, 760);
  login();
  cy.get('button[aria-label="Open navigation"]').then($button => inputAt($button, 'Input.dispatchTouchEvent', { type: 'touchStart' }));
  cy.get('button[aria-label="Open navigation"]').then($button => inputAt($button, 'Input.dispatchTouchEvent', { type: 'touchEnd' }));
  cy.get('[role=dialog]').should('be.visible').and('have.attr', 'aria-modal', 'true');
  cy.get('[role=dialog] nav a, [role=dialog] button, header a[aria-label="JobTrack home"]').each($control => {
    expect($control[0].getBoundingClientRect().height).to.be.at.least(44);
  });
  cy.get('[role=dialog] nav a').should('have.length', 10).last().focus();
  cy.press(Cypress.Keyboard.Keys.TAB);
  cy.focused().should('have.attr', 'aria-label', 'JobTrack home');
  cy.press(Cypress.Keyboard.Keys.ESC);
  cy.get('button[aria-label="Open navigation"]').should('have.attr', 'aria-expanded', 'false');
  cy.get('[role=dialog]').should('not.exist');
  // A keyboard-opened drawer restores the trigger's focus on dismissal.
  cy.get('button[aria-label="Open navigation"]').focus().should('be.focused');
  enterOnMenu();
  cy.get('[role=dialog]').should('be.visible');
  cy.press(Cypress.Keyboard.Keys.ESC);
  cy.get('[role=dialog]').should('not.exist');
  cy.get('button[aria-label="Open navigation"]').should('be.focused');
  enterOnMenu();
  cy.get('button[aria-label="Open navigation"]').should('have.attr', 'aria-expanded', 'true');
  cy.get('[role=dialog]').should('be.visible');
  cy.contains('[role=dialog] nav a', /^Saved jobs$/).click();
  cy.location('pathname').should('eq', '/applicant/saved-jobs');
  cy.get('button[aria-label="Open navigation"]').should('have.attr', 'aria-expanded', 'false');
});

for (const [role, destinations] of Object.entries({
  applicant: [['Search jobs', 'jobs'], ['View applications', 'applications'], ['Your profile & skills', 'profile'], ['Your resumes', 'resumes'], ['Saved opportunities', 'saved-jobs']],
  employer: [['My jobs', 'jobs'], ['View applications', 'applications'], ['Your company', 'company'], ['Employer verification', 'verification'], ['Your offers', 'offers']],
  admin: [['Review employers', 'verification'], ['View analytics', 'analytics'], ['Job reports', 'moderation'], ['Audit trail', 'audit-log'], ['Notifications', 'notifications']],
})) it(`JT-101 ${role} home links lead to existing authorized features`, () => {
  cy.viewport(1280, 900);
  login(role);
  for (const [label, path] of destinations) {
    cy.contains('.workspace-content a', label).click();
    cy.location('pathname').should('eq', `/${role}/${path}`);
    cy.get('header a[aria-label="JobTrack home"]').click();
    cy.location('pathname').should('eq', `/${role}`);
  }
});

for (const path of ['/login', '/register']) it(`JT-101 ${path} artwork and footer stay in separate flow at every width`, () => {
  cy.visit(path);
  cy.get('.auth-illustration').should('be.visible');
  for (const width of [320, 390, 768, 1024, 1280]) {
    cy.viewport(width, 900);
    cy.document().then(doc => {
      const art = doc.querySelector('.auth-illustration').getBoundingClientRect();
      const footer = doc.querySelector('.auth-story-footer').getBoundingClientRect();
      expect(footer.top).to.be.at.least(art.bottom);
      expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth);
    });
    cy.get('main h1').should('be.visible');
  }
});
