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
    // JT-110: no press movement; motion only answers an action.
    expect($button[0].matches(':active')).to.eq(true);
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
  cy.get('[role=dialog] nav a').should('have.length', 6).last().focus();
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
  cy.contains('[role=dialog] nav a', /^Saved$/).click();
  cy.location('pathname').should('eq', '/applicant/saved');
  cy.get('button[aria-label="Open navigation"]').should('have.attr', 'aria-expanded', 'false');
});

// JT-110 home pages. The stubs give an applicant a profile but no resume, and an unverified employer with no request.
for (const [role, destinations] of Object.entries({
  applicant: [[/^Find jobs$/, 'jobs'], [/^All applications$/, 'applications'], [/^All saved jobs$/, 'saved'], [/^Upload resume$/, 'profile']],
  employer: [[/^Post a job$/, 'jobs/new'], [/^Request verification$/, 'verification'], [/^All jobs$/, 'jobs'], [/^All applicants$/, 'applications']],
  admin: [['Pending verification', 'verification'], ['Open reports', 'moderation'], ['Count hiring events', 'analytics'], ['Every domain event', 'audit-log']],
})) it(`JT-110 ${role} home links lead to existing authorized features`, () => {
  cy.viewport(1280, 900);
  login(role);
  for (const [label, path] of destinations) {
    cy.contains('.workspace-content a', label).click();
    cy.location('pathname').should('eq', `/${role}/${path}`);
    cy.get('header a[aria-label="JobTrack home"]').click();
    cy.location('pathname').should('eq', `/${role}`);
  }
});

for (const path of ['/login', '/register']) it(`JT-110 ${path} card fits every width without horizontal scrolling`, () => {
  cy.visit(path);
  for (const width of [320, 390, 768, 1024, 1280]) {
    cy.viewport(width, 900);
    cy.get('main form').should($form => {
      const doc = $form[0].ownerDocument;
      const card = $form[0].getBoundingClientRect();
      expect(card.left).to.be.at.least(0);
      expect(card.right).to.be.at.most(doc.documentElement.clientWidth);
      expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth);
    });
    cy.get('main h1').should('be.visible');
  }
});
