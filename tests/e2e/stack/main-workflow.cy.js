// JT-95: the main workflow through the real UI against the running Docker Compose
// stack (functions profile on). No cy.intercept stubs: every request reaches the
// real gateway and services. Run with npm run test:stack (see tests/e2e/README.md).
//
// Admin login comes from CYPRESS_ADMIN_EMAIL and CYPRESS_ADMIN_PASSWORD, read with
// cy.env() so the values stay out of the browser and the command log (Cypress 16).
// All other accounts are synthetic and unique per run, so the spec can be repeated.

const run = Date.now().toString(36);
const password = `Jt95-${run}-pass`;
const employer = { email: `jt95-employer-${run}@jobtrack.test`, name: `JT-95 Employer ${run}` };
const applicant = { email: `jt95-applicant-${run}@jobtrack.test`, name: `JT-95 Applicant ${run}` };
const company = `JT-95 Synthetic Co ${run}`;
const registration = `JT95-${run}`.toUpperCase();
const resumeName = `jt95-${run}.pdf`;
// A one-page synthetic PDF that passes the resume-validator checks.
const resumePdf = 'JVBERi0xLjQKMSAwIG9iago8PCAvVHlwZSAvQ2F0YWxvZyAvUGFnZXMgMiAwIFIgPj4KZW5kb2JqCjIgMCBvYmoKPDwgL1R5cGUgL1BhZ2VzIC9LaWRzIFszIDAgUl0gL0NvdW50IDEgPj4KZW5kb2JqCjMgMCBvYmoKPDwgL1R5cGUgL1BhZ2UgL1BhcmVudCAyIDAgUiAvTWVkaWFCb3ggWzAgMCA2MTIgNzkyXSAvUmVzb3VyY2VzIDw8IC9Gb250IDw8IC9GMSA0IDAgUiA+PiA+PiAvQ29udGVudHMgNSAwIFIgPj4KZW5kb2JqCjQgMCBvYmoKPDwgL1R5cGUgL0ZvbnQgL1N1YnR5cGUgL1R5cGUxIC9CYXNlRm9udCAvSGVsdmV0aWNhID4+CmVuZG9iago1IDAgb2JqCjw8IC9MZW5ndGggNjUgPj4Kc3RyZWFtCkJUIC9GMSAxMiBUZiA3MiA3MjAgVGQgKEpULTk1IHN5bnRoZXRpYyBlbmQgdG8gZW5kIHJlc3VtZSkgVGogRVQKZW5kc3RyZWFtCmVuZG9iagp4cmVmCjAgNgowMDAwMDAwMDAwIDY1NTM1IGYgCjAwMDAwMDAwMDkgMDAwMDAgbiAKMDAwMDAwMDA1OCAwMDAwMCBuIAowMDAwMDAwMTE1IDAwMDAwIG4gCjAwMDAwMDAyNDEgMDAwMDAgbiAKMDAwMDAwMDMxMSAwMDAwMCBuIAp0cmFpbGVyCjw8IC9TaXplIDYgL1Jvb3QgMSAwIFIgPj4Kc3RhcnR4cmVmCjQyNQolJUVPRgo=';
const state = {};

// Waits for slow, event-driven updates: runs action, then checks the page, up to tries times.
function until(description, action, check, tries = 30) {
  action();
  cy.wait(1000);
  cy.get('body').then(body => {
    if (check(body)) return;
    if (tries <= 1) throw new Error(`Timed out waiting for: ${description}`);
    until(description, action, check, tries - 1);
  });
}
const hasExact = (body, selector, text) => body.find(selector).filter((_, el) => el.textContent.trim() === text).length > 0;

// Retries a submit while the service has not yet received the event it needs
// (for example "not available yet"), until the success text is on the page.
function submitUntil(description, submit, success, tries = 15) {
  submit();
  outcome(success, 30).then(result => {
    if (result === 'success') return;
    if (tries <= 1) throw new Error(`${description} failed: ${result}`);
    cy.wait(2000);
    submitUntil(description, submit, success, tries - 1);
  });
}
// Yields 'success' once the success text shows, or the text of an error alert.
function outcome(success, polls) {
  return cy.get('body').then(body => {
    if (body.text().includes(success)) return 'success';
    const error = body.find('.MuiAlert-colorError').text();
    if (error) return error;
    if (polls <= 1) return 'no result on the page';
    cy.wait(500);
    return outcome(success, polls - 1);
  });
}

const pad = n => String(n).padStart(2, '0');
// Value for <input type="datetime-local">: daysAhead from now at hour:00, local time.
function localInput(daysAhead, hour) {
  const d = new Date(Date.now() + daysAhead * 86400000);
  return `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(hour)}:00`;
}

// Cypress types key by key; keep typed text short (under about 50 characters),
// because long typing trips React's nested update limit inside Cypress (JT-83).
function field(label) {
  return cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
}
function chooseOption(label, option) {
  cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('[role=combobox]').click();
  cy.contains('[role=option]', option).click();
}
function nav(label) {
  cy.get('nav[aria-label=Main]').contains('a', new RegExp(`^${label}$`)).click();
}
// Pages that left the JT-110 menus keep their routes until their own phase.
function go(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
function login(email, secret, landing) {
  cy.visit('/login');
  cy.get('input[name=email]').type(email);
  cy.get('input[name=password]').type(secret, { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname', { timeout: 15000 }).should('eq', landing);
}
function register(account, role) {
  cy.visit('/register');
  cy.get('input[name=fullName]').type(account.name);
  cy.get('input[name=email]').type(account.email);
  cy.get('input[name=password]').type(password, { log: false });
  cy.get('input[name=confirmPassword]').type(password, { log: false });
  if (role === 'Employer') chooseOption('Account type', 'Employer');
  cy.get('button[type=submit]').click();
  cy.contains('Account created. Log in to continue.', { timeout: 15000 }).should('be.visible');
}
// Opens one of the application card buttons for this run's application. The ID is
// read when the command runs, not when it is queued, because step 06 sets it.
function openApplication(button) {
  // Employers reach applications through Applicants (JT-110).
  cy.location('pathname').then(path => nav(path.startsWith('/employer') ? 'Applicants' : 'Applications'));
  cy.then(() => {
    until('the application in the list', () => cy.contains('button', 'Refresh applications').click(),
      body => body.text().includes(state.applicationId));
    cy.contains('p', `Application: ${state.applicationId}`).parents('.MuiCard-root').within(() => {
      cy.contains('a', new RegExp(`^${button}$`)).click();
    });
  });
}
// Waits on the Status page until the current status is the expected one.
function waitForStatus(label) {
  openApplication('Status');
  until(`status ${label}`, () => cy.contains('button', 'Refresh status').click(),
    body => hasExact(body, '.MuiChip-label', label));
}

describe('JT-95 main workflow against the running stack', () => {
  it('01 employer registers and creates a company profile', () => {
    register(employer, 'Employer');
    login(employer.email, password, '/employer');
    nav('Company');
    field('Company name').type(company);
    cy.contains('button', 'Create profile').click();
    cy.contains('Company profile created', { timeout: 15000 }).should('be.visible');
    cy.screenshot('01-company-profile');
  });

  it('02 employer requests verification', () => {
    login(employer.email, password, '/employer');
    go('/employer/verification');
    field('Registration number').type(registration);
    cy.contains('button', 'Send request').click();
    cy.contains('Request sent.', { timeout: 15000 }).should('be.visible');
  });

  it('03 admin approves the verification request', () => {
    cy.env(['ADMIN_EMAIL', 'ADMIN_PASSWORD']).then(({ ADMIN_EMAIL, ADMIN_PASSWORD }) => {
      // Checked without printing the values.
      if (!ADMIN_EMAIL || !ADMIN_PASSWORD) throw new Error('Set CYPRESS_ADMIN_EMAIL and CYPRESS_ADMIN_PASSWORD (tests/e2e/README.md)');
      cy.visit('/login');
      cy.get('input[name=email]').type(ADMIN_EMAIL, { log: false });
      cy.get('input[name=password]').type(ADMIN_PASSWORD, { log: false });
      cy.get('button[type=submit]').click();
      cy.location('pathname', { timeout: 15000 }).should('eq', '/admin');
    });
    nav('Verification');
    const findRequest = (tries = 20) => cy.get('body').then(body => {
      if (body.text().includes(`Registration number: ${registration}`)) return;
      if (tries <= 1 || !body.find('nav[aria-label=Pages] button:contains("Next"):enabled').length) throw new Error('verification request not listed');
      cy.contains('nav[aria-label=Pages] button', /^Next$/).click();
      cy.wait(500);
      findRequest(tries - 1);
    });
    findRequest();
    cy.contains('h2', `Registration number: ${registration}`).parents('li').first().within(() => {
      cy.contains('button', 'Approve').click();
    });
    cy.contains('button', 'Confirm approval').click();
    cy.contains('Request approved.', { timeout: 15000 }).should('be.visible');
    cy.screenshot('03-admin-approved');
  });

  it('04 employer sees Verified, creates and publishes a job', () => {
    login(employer.email, password, '/employer');
    until('company Verified', () => nav('Company'), body => hasExact(body, '.MuiChip-label', 'Verified'));
    nav('Jobs');
    cy.contains('a', 'Create job').click();
    field('Title').type(`JT-95 Developer ${run}`);
    field('Deadline').type(localInput(3, 17));
    cy.contains('button', 'Create draft').should('be.enabled').click();
    cy.location('pathname', { timeout: 15000 }).should('match', /^\/employer\/jobs\/[0-9a-f-]{36}$/).then(path => {
      state.jobId = path.split('/').pop();
    });
    cy.contains('Status: Draft').should('be.visible');
    submitUntil('publish job', () => cy.get('body').then(body => {
      // After a failed attempt the confirmation dialog is still open.
      if (!body.find('button:contains("Confirm publish")').length) cy.contains('button', 'Publish job').click();
      cy.contains('button', 'Confirm publish').click();
    }), 'Job published.');
    cy.contains('Status: Published', { timeout: 15000 }).should('be.visible');
    cy.screenshot('04-job-published');
  });

  it('05 applicant registers and uploads a resume that is validated', () => {
    register(applicant, 'Applicant');
    login(applicant.email, password, '/applicant');
    go('/applicant/resumes');
    cy.get('input[type=file][name=resume]').selectFile({ contents: Cypress.Buffer.from(resumePdf, 'base64'), fileName: resumeName, mimeType: 'application/pdf' }, { force: true });
    cy.contains(`${resumeName} uploaded`, { timeout: 15000 }).should('be.visible');
    until('resume Validated', () => cy.contains('button', 'Refresh resumes').click(),
      body => hasExact(body, '.MuiChip-label', 'Validated'));
    cy.screenshot('05-resume-validated');
  });

  it('06 applicant applies for the job', () => {
    login(applicant.email, password, '/applicant');
    go('/applicant/apply');
    field('Job ID').type(state.jobId);
    cy.contains('button', 'Load job').click();
    cy.contains(`JT-95 Developer ${run}`, { timeout: 15000 }).should('be.visible');
    chooseOption('Validated resume', resumeName);
    cy.contains('button', 'Submit application').click();
    cy.contains('Application submitted. ID:', { timeout: 15000 }).invoke('text').then(text => {
      state.applicationId = text.match(/[0-9a-f-]{36}/)[0];
    });
    waitForStatus('Submitted');
  });

  it('07 employer reviews and shortlists', () => {
    login(employer.email, password, '/employer');
    openApplication('Review');
    chooseOption('Score', /^4$/);
    cy.contains('label', 'Shortlist').click();
    submitUntil('complete review', () => {
      cy.contains('button', 'Complete review').click();
      cy.contains('button', 'Confirm review').click();
    }, 'Review completed');
    cy.contains('Review completed').should('be.visible');
    cy.screenshot('07-review-completed');
    waitForStatus('Shortlisted');
  });

  it('08 employer schedules an interview', () => {
    login(employer.email, password, '/employer');
    openApplication('Interviews');
    field('Date and time').type(localInput(2, 10));
    field('Details').type('Room 2, JT-95 synthetic interview');
    submitUntil('schedule interview', () => cy.contains('button', 'Schedule interview').click(), 'Interview scheduled for');
    cy.contains('Room 2, JT-95 synthetic interview').should('be.visible');
    waitForStatus('Interview');
  });

  it('09 employer makes an offer', () => {
    login(employer.email, password, '/employer');
    openApplication('Make offer');
    field('Offer terms').type('JT-95 offer: start next month.');
    submitUntil('make offer', () => cy.contains('button', 'Make offer').click(), 'Offer made.');
    cy.contains('Offer made.').invoke('text').then(text => {
      state.offerId = text.match(/[0-9a-f-]{36}/)[0];
    });
    cy.screenshot('09-offer-made');
    waitForStatus('Offered');
  });

  it('10 applicant accepts the offer', () => {
    login(applicant.email, password, '/applicant');
    go('/applicant/offers');
    field('Offer ID').type(state.offerId);
    cy.contains('button', 'Load offer').click();
    cy.contains('button', 'Accept offer', { timeout: 15000 }).click();
    cy.contains('button', 'Confirm decision').click();
    cy.contains('button', 'Accept offer').should('not.exist');
  });

  it('11 status history shows the whole workflow', () => {
    login(applicant.email, password, '/applicant');
    waitForStatus('Accepted');
    const expected = [['None', 'Submitted'], ['Submitted', 'Under review'], ['Under review', 'Shortlisted'],
      ['Shortlisted', 'Interview'], ['Interview', 'Offered'], ['Offered', 'Accepted']];
    cy.get('table[aria-label="Status history"] tbody tr').should('have.length', expected.length).each((row, i) => {
      cy.wrap(row).find('td').eq(0).should('have.text', expected[i][0]);
      cy.wrap(row).find('td').eq(1).should('have.text', expected[i][1]);
    });
    cy.screenshot('11-status-history');
    cy.then(() => cy.log(`job ${state.jobId}, application ${state.applicationId}, offer ${state.offerId}`));
  });
});
