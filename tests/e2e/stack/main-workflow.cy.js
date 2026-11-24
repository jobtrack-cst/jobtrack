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
// Switching user inside one step: log out through the account menu first, because a
// signed-in session (kept in sessionStorage) sends /login straight back to its home page.
function logOut() {
  cy.get('#account-menu-button').click();
  cy.contains('#account-menu button', 'Log out').click();
  cy.location('pathname').should('eq', '/login');
  cy.get('#account-menu-button').should('not.exist');
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
// JT-111: the employer opens this run's application from Applicants, filtered by this
// run's job, by the applicant's name (stored at submission, JT-114). No IDs are typed.
function openApplicantPage() {
  nav('Applicants');
  cy.get('form[role=search] select option').should('contain', `JT-95 Developer ${run}`);
  cy.get('form[role=search] select').select(`JT-95 Developer ${run}`);
  cy.get('form[role=search]').contains('button', 'Show applicants').click();
  cy.contains('ul[aria-label=Applicants] > li', applicant.name, { timeout: 15000 }).contains('a', /^View$/).click();
  cy.location('pathname').should('match', /^\/employer\/applicants\/[0-9a-f-]{36}$/);
  cy.contains('main h1', applicant.name).should('be.visible');
}
// Waits on the employer application page until the status rail shows the label as current.
function waitForEmployerStatus(label) {
  until(`employer status ${label}`, () => cy.get('section[aria-label=Status]').contains('button', /^Refresh$/).click(),
    body => body.find(`section[aria-label=Status] ol[aria-label*=", ${label}, current"]`).length > 0);
}
// Runs one action dialog on the application page: opens it with its button (unless it is
// still open after a failed attempt, for example "not available yet" while an event is on
// its way), fills it on first open, and confirms.
function dialogAction(open, fill, confirm) {
  cy.get('body').then(body => {
    if (!body.find('[role=dialog]').length) {
      cy.contains('main button', new RegExp(`^${open}$`), { timeout: 15000 }).click();
      cy.get('[role=dialog]').should('be.visible');
      // Not scoped to the dialog: a select's options open in a popover outside it.
      fill();
    }
    cy.get('[role=dialog]').contains('button', new RegExp(`^${confirm}$`)).click();
  });
}

// JT-112: the applicant follows the application on its own page. Status events arrive
// with a delay, so the page is reloaded until the Status section shows the label.
function waitForApplicantStatus(label, final = false) {
  cy.then(() => go(`/applicant/applications/${state.applicationId}`));
  until(`applicant status ${label}`, () => cy.reload(), body => final
    ? hasExact(body, 'section[aria-label=Status] .MuiChip-label', label)
    : body.find(`section[aria-label=Status] ol[aria-label*=", ${label}, current"]`).length > 0);
}

// Evidence screenshots capture only the viewport: full-page captures time out on slower
// machines. The relevant part is scrolled into view first, clear of the sticky top bar.
function evidence(name, target) {
  target().scrollIntoView({ offset: { top: -120, left: 0 } }).should('be.visible');
  cy.screenshot(name, { capture: 'viewport' });
}

describe('JT-95 main workflow against the running stack', () => {
  it('01 employer registers and creates a company profile', () => {
    register(employer, 'Employer');
    login(employer.email, password, '/employer');
    nav('Company');
    field('Company name').type(company);
    cy.contains('button', 'Create profile').click();
    cy.contains('Company profile created', { timeout: 15000 }).should('be.visible');
    // JT-111: the saved profile is read-only with Edit (JT-109 pattern).
    cy.contains('button', 'Edit profile').should('be.visible');
    cy.get('section[aria-label="Company profile"]').should('contain', company);
    evidence('01-company-profile', () => cy.contains('Company profile created'));
  });

  it('02 employer requests verification on the Company page', () => {
    login(employer.email, password, '/employer');
    // JT-111: the request form is in the Verification section of Company.
    nav('Company');
    cy.get('section[aria-label=Verification]').within(() => {
      field('Registration number').type(registration);
      cy.contains('button', 'Send request').click();
      cy.contains('Request sent.', { timeout: 15000 }).should('be.visible');
      cy.contains('.MuiChip-label', /^Pending$/).should('be.visible');
    });
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
    // The Pending tab lists the oldest first, 20 per page, so pending requests left by
    // earlier runs can push this run's request to a later page. Each page is read only
    // after it has loaded: the list (or the empty state) is shown and the Pager names its range.
    const settled = page => {
      cy.contains('main h1', /^Verification$/).should('be.visible');
      cy.get('main [role=status][aria-label="Loading requests"]', { timeout: 15000 }).should('not.exist');
      cy.get('main ul[aria-label="Verification requests"], main [role=status]:contains("No requests here")', { timeout: 15000 }).should('exist');
      if (page > 0) cy.contains('nav[aria-label=Pages] [role=status]', `Showing ${page * 20 + 1} to`, { timeout: 15000 }).should('exist');
    };
    const findRequest = (page = 0) => {
      settled(page);
      cy.get('main').then(main => {
        if (main.find('h2').filter((_, el) => el.textContent.trim() === `Registration number: ${registration}`).length) return;
        if (page >= 49 || !main.find('nav[aria-label=Pages] button:contains("Next"):enabled').length) throw new Error('verification request not listed');
        cy.contains('nav[aria-label=Pages] button', /^Next$/).click();
        findRequest(page + 1);
      });
    };
    findRequest();
    cy.contains('h2', `Registration number: ${registration}`).parents('li').first().within(() => {
      cy.contains('button', 'Approve').click();
    });
    cy.contains('button', 'Confirm approval').click();
    cy.contains('Request approved.', { timeout: 15000 }).should('be.visible');
    evidence('03-admin-approved', () => cy.contains('Request approved.'));
  });

  it('04 employer sees Verified, creates and publishes a job', () => {
    login(employer.email, password, '/employer');
    nav('Company');
    until('company Verified', () => cy.get('section[aria-label=Verification]').contains('button', /^Refresh$/).click(),
      body => hasExact(body, 'section[aria-label=Verification] .MuiChip-label', 'Verified'));
    nav('Jobs');
    cy.contains('main a', 'Post a job').click();
    field('Title').type(`JT-95 Developer ${run}`);
    field('Deadline').type(localInput(3, 17));
    cy.contains('button', 'Create draft').should('be.enabled').click();
    cy.location('pathname', { timeout: 15000 }).should('match', /^\/employer\/jobs\/[0-9a-f-]{36}$/).then(path => {
      state.jobId = path.split('/').pop();
    });
    // JT-111: the new job opens read-only as a draft, with Publish beside Edit.
    cy.contains('Draft created.').should('be.visible');
    cy.contains('main .MuiChip-label', /^Draft$/).should('be.visible');
    submitUntil('publish job', () => cy.get('body').then(body => {
      // After a failed attempt the confirmation dialog is still open.
      if (!body.find('button:contains("Confirm publish")').length) cy.contains('button', 'Publish job').click();
      cy.contains('button', 'Confirm publish').click();
    }), 'Job published.');
    cy.contains('main .MuiChip-label', /^Published$/, { timeout: 15000 }).should('be.visible');
    nav('Jobs');
    cy.contains('ul[aria-label="Active jobs"] > li', `JT-95 Developer ${run}`).should('be.visible');
    evidence('04-job-published', () => cy.contains('ul[aria-label="Active jobs"] > li', `JT-95 Developer ${run}`));
  });

  it('05 applicant registers and uploads a resume that is validated', () => {
    register(applicant, 'Applicant');
    login(applicant.email, password, '/applicant');
    go('/applicant/resumes');
    cy.get('input[type=file][name=resume]').selectFile({ contents: Cypress.Buffer.from(resumePdf, 'base64'), fileName: resumeName, mimeType: 'application/pdf' }, { force: true });
    cy.contains(`${resumeName} uploaded`, { timeout: 15000 }).should('be.visible');
    until('resume Validated', () => cy.contains('button', /^Refresh$/).click(),
      body => hasExact(body, '.MuiChip-label', 'Validated'));
    evidence('05-resume-validated', () => cy.contains('main li', resumeName));
  });

  it('06 applicant applies for the job', () => {
    login(applicant.email, password, '/applicant');
    // Applicants find the job by its title; job-search builds its read model from events.
    nav('Find jobs');
    field('Job title').type(`JT-95 Developer ${run}`);
    until('the job in the search', () => cy.get('form[role=search]').contains('button', /^Search$/).click(),
      body => body.text().includes(`JT-95 Developer ${run}`) && !body.text().includes('No open roles match'));
    cy.contains('ul[aria-label=Jobs] > li', `JT-95 Developer ${run}`).contains('a', 'View job').click();
    cy.contains('main button', /^Apply$/, { timeout: 15000 }).should('be.enabled').click();
    cy.get('[role=dialog]').contains('label', resumeName).click();
    cy.get('[role=dialog]').contains('button', 'Send application').click();
    cy.contains('Application sent.', { timeout: 15000 }).should('be.visible');
    cy.contains('a', 'View application', { timeout: 15000 }).click();
    cy.location('pathname').should('match', /^\/applicant\/applications\/[0-9a-f-]{36}$/).then(path => {
      state.applicationId = path.split('/').pop();
    });
    evidence('06-applied', () => cy.get('section[aria-label=Status]'));
    waitForApplicantStatus('Submitted');
  });

  it('07 employer reviews and shortlists on the application page', () => {
    login(employer.email, password, '/employer');
    openApplicantPage();
    // The applicant's profile area and the history are on the same page (JT-111).
    cy.get('section[aria-label=Applicant]').should('contain', applicant.name);
    waitForEmployerStatus('Submitted');
    submitUntil('complete review', () => dialogAction('Review application', () => {
      chooseOption('Score', '4 of 5');
      cy.contains('label', 'Shortlist').click();
    }, 'Shortlist applicant'), 'Review saved.');
    cy.get('section[aria-label=Review]').should('contain', 'Score 4 of 5');
    evidence('07-review-completed', () => cy.get('section[aria-label=Review]'));
    waitForEmployerStatus('Shortlisted');
  });

  it('08 employer schedules an interview', () => {
    login(employer.email, password, '/employer');
    openApplicantPage();
    waitForEmployerStatus('Shortlisted');
    submitUntil('schedule interview', () => dialogAction('Schedule interview', () => {
      field('Date and time').type(localInput(2, 10));
      field('Details').type('Room 2, JT-95 synthetic interview');
    }, 'Schedule interview'), 'Interview scheduled for');
    cy.get('section[aria-label=Interviews]').should('contain', 'Room 2, JT-95 synthetic interview');
    waitForEmployerStatus('Interview');
  });

  it('09 employer makes an offer', () => {
    login(employer.email, password, '/employer');
    openApplicantPage();
    waitForEmployerStatus('Interview');
    submitUntil('make offer', () => dialogAction('Make offer', () => {
      field('Offer terms').type('JT-95 offer: start next month.');
    }, 'Send offer'), 'Offer sent.');
    cy.get('section[aria-label=Offer]').should('contain', 'JT-95 offer: start next month.').and('contain', 'Awaiting reply');
    evidence('09-offer-made', () => cy.get('section[aria-label=Offer]'));
    waitForEmployerStatus('Offer');
  });

  // JT-113: before the offer is answered (a final application is view only), the employer
  // sends one message from the Messages inbox and both sides see it there.
  it('10 employer and applicant see one message in the Messages inbox', () => {
    const text = `JT-113 hello ${run}`;
    const conversation = name => cy.contains('ul[aria-label=Conversations] > li', name, { timeout: 15000 });
    login(employer.email, password, '/employer');
    nav('Messages');
    cy.location('pathname').should('eq', '/employer/messages');
    conversation(applicant.name).find('a').click();
    cy.location('pathname').should('match', /^\/employer\/messages\/[0-9a-f-]{36}$/);
    cy.get(`section[aria-label="Conversation with ${applicant.name}"]`).within(() => {
      field('Message').type(text);
      cy.contains('button', 'Send message').click();
      cy.get('[role=log]', { timeout: 15000 }).should('contain', text);
    });
    // Read again from messaging, not only the message just sent.
    cy.reload();
    conversation(applicant.name).should('contain', `You: ${text}`).and('contain', `JT-95 Developer ${run}`);
    evidence('10-employer-inbox', () => cy.get('ul[aria-label=Conversations]'));

    logOut();
    login(applicant.email, password, '/applicant');
    nav('Messages');
    conversation(company).should('contain', text).and('not.contain', 'You:').and('contain', `JT-95 Developer ${run}`).find('a').click();
    cy.get(`section[aria-label="Conversation with ${company}"] [role=log]`, { timeout: 15000 }).should('contain', text).and('contain', company);
    evidence('10-applicant-inbox', () => cy.get(`section[aria-label="Conversation with ${company}"]`));
  });

  it('11 applicant accepts the offer', () => {
    login(applicant.email, password, '/applicant');
    // The offer is loaded on the application page with GetOfferByApplication (JT-108).
    cy.then(() => go(`/applicant/applications/${state.applicationId}`));
    until('the offer on the application page', () => cy.reload(),
      body => body.find('section[aria-label=Offer] button:contains("Accept offer")').length > 0);
    cy.get('section[aria-label=Offer]').contains('button', 'Accept offer').click();
    cy.get('[role=dialog]').contains('button', 'Accept offer').click();
    cy.contains('Offer accepted.', { timeout: 15000 }).should('be.visible');
    cy.get('section[aria-label=Offer]').contains('button', 'Accept offer').should('not.exist');
  });

  it('12 status history shows the whole workflow', () => {
    login(applicant.email, password, '/applicant');
    waitForApplicantStatus('Accepted', true);
    const expected = [['None', 'Submitted'], ['Submitted', 'Under review'], ['Under review', 'Shortlisted'],
      ['Shortlisted', 'Interview'], ['Interview', 'Offered'], ['Offered', 'Accepted']];
    // Applicant page: each row reads "<new status> from <previous status>", oldest first. The
    // first row still says "from None" there until JT-115 item 3 switches it to StatusHistory.
    cy.get('ul[aria-label="Status history"] > li').should('have.length', expected.length).each((row, i) => {
      cy.wrap(row).should('contain', `${expected[i][1]} from ${expected[i][0]}`);
    });
    evidence('12-status-history',() => cy.get('ul[aria-label="Status history"]'));

    // Employer page (JT-111): the same history, the first entry without a "from" part, and no actions left.
    logOut();
    login(employer.email, password, '/employer');
    openApplicantPage();
    cy.get('section[aria-label=Status] .MuiChip-label').filter((_, el) => el.textContent.trim() === 'Accepted').should('have.length', 1);
    cy.get('ul[aria-label="Status history"] > li').should('have.length', expected.length).each((row, i) => {
      if (i === 0) cy.wrap(row).invoke('text').should('match', /^Submitted/).and('not.contain', 'from');
      else cy.wrap(row).should('contain', `${expected[i][1]} from ${expected[i][0]}`);
    });
    for (const action of ['Review application', 'Schedule interview', 'Schedule another interview', 'Make offer']) {
      cy.contains('main button', new RegExp(`^${action}$`)).should('not.exist');
    }
    cy.get('section[aria-label=Conversation]').should('contain', 'view only');
    evidence('12-employer-history',() => cy.get('ul[aria-label="Status history"]'));
    cy.then(() => cy.log(`job ${state.jobId}, application ${state.applicationId}`));
  });
});
