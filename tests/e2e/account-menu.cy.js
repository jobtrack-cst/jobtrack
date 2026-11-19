import { openAccountMenu } from './account-menu';

const gateway = 'http://localhost:8000';
const fullName = 'Pema Wangchuk Choden '.repeat(8).trim();
const profiles = { applicant: ['My profile', '/applicant/profile'], employer: ['Company profile', '/employer/company'] };

function login(role = 'applicant') {
  const user = { userId: '11111111-1111-4111-8111-111111111111', fullName, email: 'synthetic@example.test', role: `ROLE_${role.toUpperCase()}` };
  cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
  cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} });
  cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { userId: user.userId, education: [], skills: [] } });
  cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile: { employerId: user.userId, companyName: 'Synthetic Company' } });
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt101-account', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('synthetic-password', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', `/${role}`);
}

function closed() {
  cy.get('#account-menu').should('not.exist');
  cy.get('#account-menu-button').should('have.attr', 'aria-expanded', 'false');
}

// Trusted Chromium input verifies the real pointer and touch states in the AUT frame.
function inputAt($element, command, params) {
  cy.window().then(win => {
    const frame = (win.frameElement || window.top.document.querySelector('iframe.aut-iframe')).getBoundingClientRect();
    const scale = frame.width / win.innerWidth;
    const rect = $element[0].getBoundingClientRect();
    const x = frame.left + (rect.left + rect.width / 2) * scale;
    const y = frame.top + (rect.top + rect.height / 2) * scale;
    return Cypress.automation('remote:debugger:protocol', { command, params: command === 'Input.dispatchTouchEvent'
      ? { ...params, touchPoints: params.type === 'touchEnd' ? [] : [{ x, y }] } : { ...params, x, y } });
  });
}

function tap(selector) {
  cy.get(selector).then($element => inputAt($element, 'Input.dispatchTouchEvent', { type: 'touchStart' }));
  cy.get(selector).then($element => inputAt($element, 'Input.dispatchTouchEvent', { type: 'touchEnd' }));
}

function keypress(key) {
  const [code, virtualKey, text] = {
    Enter: ['Enter', 13, '\r'], Space: ['Space', 32, ' '], ArrowDown: ['ArrowDown', 40],
    ArrowUp: ['ArrowUp', 38], Escape: ['Escape', 27],
  }[key];
  cy.window().then(win => win.focus());
  for (const type of ['keyDown', 'keyUp']) cy.then(() => Cypress.automation('remote:debugger:protocol', {
    command: 'Input.dispatchKeyEvent', params: { type, key: key === 'Space' ? ' ' : key, code, windowsVirtualKeyCode: virtualKey,
      ...(type === 'keyDown' && text ? { text, unmodifiedText: text } : {}) },
  }));
}

for (const role of ['applicant', 'employer', 'admin']) {
  it(`JT-101 ${role} account shows only authorized actions, fits five widths and logs out`, () => {
    login(role);
    for (const width of [320, 390, 768, 1024, 1280]) {
      cy.viewport(width, 900);
      cy.get('#account-menu-button').should('have.attr', 'aria-haspopup', 'menu').and('contain.text', role === 'admin' ? 'Admin' : role === 'employer' ? 'Employer' : 'Applicant');
      openAccountMenu();
      cy.get('#account-menu [role=menu]').should('have.attr', 'aria-labelledby', 'account-menu-button');
      cy.get('#account-menu [role=menuitem]').should('have.length', role === 'admin' ? 1 : 2);
      cy.get('#account-menu').should('contain.text', fullName).and('contain.text', `${role === 'admin' ? 'Admin' : role === 'employer' ? 'Employer' : 'Applicant'} account`);
      cy.get('#account-menu a').should('have.length', role === 'admin' ? 0 : 1);
      if (profiles[role]) cy.contains('#account-menu a', profiles[role][0]).should('have.attr', 'href', profiles[role][1]);
      cy.get('#account-menu .MuiPaper-root').should($paper => {
        const rect = $paper[0].getBoundingClientRect();
        expect(rect.left).to.be.at.least(0);
        expect(rect.right).to.be.at.most(width);
      });
      cy.get('#account-menu-button, #account-menu [role=menuitem]').each($element => expect($element[0].getBoundingClientRect().height).to.be.at.least(44));
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.press(Cypress.Keyboard.Keys.ESC);
      closed();
    }
    if (profiles[role]) {
      openAccountMenu();
      cy.contains('#account-menu a', profiles[role][0]).click();
      cy.location('pathname').should('eq', profiles[role][1]);
      closed();
    }
    openAccountMenu();
    cy.contains('button', 'Log out').click();
    cy.location('pathname').should('eq', '/login');
    cy.get('#account-menu, #account-menu-button').should('not.exist');
    cy.window().should(win => expect(win.sessionStorage.getItem('jobtrack.auth')).to.eq(null));
    cy.reload();
    cy.location('pathname').should('eq', '/login');
  });

  it(`JT-101 ${role} account supports native touch and easy logout at 320px`, () => {
    cy.viewport(320, 760);
    login(role);
    tap('#account-menu-button');
    cy.get('#account-menu [role=menu]').should('be.visible');
    cy.get('#account-menu .MuiPaper-root').should('have.css', 'transform', 'none');
    tap('#account-menu button');
    cy.location('pathname').should('eq', '/login');
    cy.get('#account-menu').should('not.exist');
  });
}

it('JT-101 account keyboard supports Enter, Space, arrows, Escape and focus return', () => {
  cy.viewport(1280, 900);
  login();
  for (const key of [Cypress.Keyboard.Keys.ENTER, Cypress.Keyboard.Keys.SPACE, Cypress.Keyboard.Keys.DOWN]) {
    cy.get('#account-menu-button').focus();
    keypress(key);
    cy.get('#account-menu [role=menu]').should('be.visible');
    cy.focused().should('have.attr', 'href', '/applicant/profile');
    keypress(Cypress.Keyboard.Keys.DOWN);
    cy.focused().should('have.text', 'Log out');
    keypress(Cypress.Keyboard.Keys.UP);
    cy.focused().should('have.attr', 'href', '/applicant/profile');
    keypress(Cypress.Keyboard.Keys.ESC);
    closed();
    cy.get('#account-menu-button').should('be.focused');
  }
  cy.get('#account-menu-button').focus();
  keypress(Cypress.Keyboard.Keys.ENTER);
  cy.get('#account-menu [role=menu]').should('be.visible');
  keypress(Cypress.Keyboard.Keys.ENTER);
  cy.location('pathname').should('eq', '/applicant/profile');
  closed();
});

it('JT-101 account closes on outside click, route changes and history navigation', () => {
  login();
  openAccountMenu();
  cy.get('#account-menu .MuiBackdrop-root').click('bottomLeft');
  closed();
  openAccountMenu();
  cy.window().then(win => { win.history.pushState({}, '', '/applicant/profile'); win.dispatchEvent(new win.PopStateEvent('popstate')); });
  cy.location('pathname').should('eq', '/applicant/profile');
  closed();
  openAccountMenu();
  cy.go('back');
  cy.location('pathname').should('eq', '/applicant');
  closed();
});

it('JT-101 account has native hover, pressed and keyboard focus states with 200% text reflow', () => {
  cy.viewport(1280, 900);
  login('employer');
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchMouseEvent', params: { type: 'mouseMoved', x: 0, y: 0 } }));
  let restingColor;
  cy.get('#account-menu-button').then($button => {
    restingColor = $button[0].ownerDocument.defaultView.getComputedStyle($button[0]).backgroundColor;
    inputAt($button, 'Input.dispatchMouseEvent', { type: 'mouseMoved' });
  });
  cy.get('#account-menu-button').should($button => {
    expect($button[0].matches(':hover')).to.eq(true);
    expect($button[0].ownerDocument.defaultView.getComputedStyle($button[0]).backgroundColor).not.to.eq(restingColor);
  }).then($button => inputAt($button, 'Input.dispatchMouseEvent', { type: 'mousePressed', button: 'left', clickCount: 1 }));
  cy.get('#account-menu-button').should($button => expect($button[0].matches(':active')).to.eq(true));
  cy.then(() => Cypress.automation('remote:debugger:protocol', { command: 'Input.dispatchMouseEvent', params: { type: 'mouseReleased', x: 0, y: 0, button: 'left', clickCount: 1 } }));
  cy.get('header a[aria-label="JobTrack home"]').focus();
  cy.press(Cypress.Keyboard.Keys.TAB);
  cy.get('#account-menu-button').should('be.focused').should($button => {
    const style = $button[0].ownerDocument.defaultView.getComputedStyle($button[0]);
    expect(style.outlineStyle).to.eq('solid');
    expect(style.outlineWidth).to.eq('3px');
  });
  cy.viewport(320, 760);
  cy.document().then(doc => { doc.documentElement.style.fontSize = '200%'; });
  openAccountMenu();
  cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  cy.contains('#account-menu button', 'Log out').should('be.visible');
});

it('JT-101 session expiry dismisses an open account menu and clears stored credentials', () => {
  login('admin');
  openAccountMenu();
  cy.clock(Date.now(), ['Date']);
  cy.tick(3600001);
  cy.window().trigger('focus');
  cy.location('pathname').should('eq', '/login');
  cy.get('#account-menu, #account-menu-button').should('not.exist');
  cy.window().should(win => expect(win.sessionStorage.getItem('jobtrack.auth')).to.eq(null));
});
