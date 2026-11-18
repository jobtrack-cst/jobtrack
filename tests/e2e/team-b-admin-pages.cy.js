// JT-88 pages. Gateway responses are stubbed, so these tests check the UI and
// the requests it sends, not live services (see docs/testing/jt-88-test-cases.md).
const gateway = 'http://localhost:8000';
const people = {
  ROLE_APPLICANT: { userId: '11111111-1111-4111-8111-111111111111', email: 'pema@example.test', fullName: 'Pema', landing: '/applicant' },
  ROLE_EMPLOYER: { userId: '66666666-6666-4666-8666-666666666666', email: 'dorji@example.test', fullName: 'Dorji', landing: '/employer' },
  ROLE_ADMIN: { userId: '99999999-9999-4999-8999-999999999999', email: 'coordinator@example.test', fullName: 'Coordinator', landing: '/admin' },
};
const employerId = people.ROLE_EMPLOYER.userId;
const adminId = people.ROLE_ADMIN.userId;
const jobId = '22222222-2222-4222-8222-222222222222';
const requestId = '55555555-5555-4555-8555-555555555555';
const reportId = '77777777-7777-4777-8777-777777777777';
const timestamps = { createdAt: '2026-11-16T04:00:00Z', updatedAt: '2026-11-16T05:00:00Z' };

function login(role) {
  const { landing, ...person } = people[role];
  const user = { ...person, role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt88-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt88-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', landing);
}
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
function open(link) { cy.contains('nav a', exact(link)).click(); }
function field(label) {
  return cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
}
// Long text is set with the native value setter and one input event, as in the
// JT-78 and JT-83 tests: typing it key by key trips React's nested update limit
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
function choose(label, option) {
  cy.contains('label', new RegExp(`^${label}`)).parent().find('[role=combobox]').click();
  cy.contains('[role=option]', exact(option)).click();
}
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
const card = text => cy.contains('.MuiCard-root', text);

describe('JT-88 company profile', () => {
  const fields = { companyName: 'Druk Tech', description: 'Synthetic company', website: 'https://druktech.example', location: 'Thimphu' };
  const profile = { employerId, ...fields, verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED', timestamps };
  function openCompany(response) {
    cy.intercept('GET', `${gateway}/v1/employer/profile`, response).as('load');
    login('ROLE_EMPLOYER');
    open('Company');
    cy.wait('@load');
  }

  it('creates a profile when the employer has none, sending trimmed fields', () => {
    openCompany({ statusCode: 404, body: { code: 5, message: 'private' } });
    cy.contains('You do not have a company profile yet').should('be.visible');
    cy.contains('button', 'Create profile').should('be.disabled');
    field('Company name').type('  Druk Tech  ');
    field('Description').type('Synthetic company');
    field('Website').type('https://druktech.example');
    field('Location').type('Thimphu ');
    cy.intercept('POST', `${gateway}/v1/employer/profile`, req => {
      expect(req.body).to.deep.eq(fields);
      req.reply({ profile });
    }).as('create');
    cy.contains('button', 'Create profile').click();
    cy.wait('@create');
    cy.contains('Company profile created').should('be.visible');
    cy.contains('.MuiChip-root', 'Not verified').should('be.visible');
    cy.contains('a', 'Request verification').should('be.visible');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a rejected profile with its reason and saves changes with PUT', () => {
    const rejected = { ...profile, verificationStatus: 'VERIFICATION_STATUS_REJECTED', rejectionReason: 'Licence number not found' };
    openCompany({ profile: rejected });
    cy.contains('.MuiChip-root', 'Rejected').should('be.visible');
    cy.contains('Reason: Licence number not found').should('be.visible');
    field('Company name').should('have.value', 'Druk Tech');
    field('Location').clear().type('Phuentsholing');
    cy.intercept('PUT', `${gateway}/v1/employer/profile`, req => {
      expect(req.body).to.deep.eq({ ...fields, location: 'Phuentsholing' });
      req.reply({ profile: { ...rejected, location: 'Phuentsholing' } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
    cy.contains('Company profile saved.').should('be.visible');
    field('Location').should('have.value', 'Phuentsholing');
  });

  it('shows a verified profile without the verification link', () => {
    openCompany({ profile: { ...profile, verificationStatus: 'VERIFICATION_STATUS_VERIFIED' } });
    cy.contains('.MuiChip-root', exact('Verified')).should('be.visible');
    cy.contains('a', 'Request verification').should('not.exist');
  });

  for (const [name, change, message] of [
    ['a 151 character name', () => fill('Company name', 'x'.repeat(151)), 'at most 150'],
    ['a name of only spaces', () => fill('Company name', '   '), ''],
    ['a 2001 character description', () => fill('Description', 'x'.repeat(2001)), 'at most 2000'],
    ['a website without http or https', () => fill('Website', 'druktech.example'), 'full address'],
    ['an ftp website', () => fill('Website', 'ftp://druktech.example'), 'full address'],
    ['a 256 character website', () => fill('Website', `https://${'a'.repeat(245)}.bt`), 'at most 255'],
    ['a 101 character location', () => fill('Location', 'x'.repeat(101)), 'at most 100'],
  ]) {
    it(`blocks saving with ${name}`, () => {
      openCompany({ profile });
      field('Company name').should('have.value', 'Druk Tech');
      change();
      if (message) cy.get('[role=alert]').should('contain', message);
      cy.contains('button', 'Save profile').should('be.disabled');
    });
  }

  it('accepts boundary values: 150 character name, 2000 description, 255 website, 100 location', () => {
    openCompany({ profile });
    fill('Company name', 'x'.repeat(150));
    fill('Description', 'x'.repeat(2000));
    fill('Website', `https://${'a'.repeat(244)}.bt`);
    fill('Location', 'x'.repeat(100));
    field('Company name').should('have.value', 'x'.repeat(150));
    cy.get('[role=alert]').should('not.exist');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a fixed message for a rejected save and never the server text', () => {
    cy.intercept('PUT', `${gateway}/v1/employer/profile`, { statusCode: 400, body: { code: 3, message: 'private-input' } });
    openCompany({ profile });
    cy.contains('button', 'Save profile').click();
    cy.get('[role=alert]').should('contain', 'The profile was not saved').and('not.contain', 'private-input');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a load failure and retries', () => {
    openCompany({ statusCode: 503, body: {} });
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile });
    cy.contains('button', 'Try again').click();
    field('Company name').should('have.value', 'Druk Tech');
  });
});

describe('JT-88 employer verification request', () => {
  const pending = { requestId, employerId, registrationNumber: 'REG-123', notes: 'Trade licence', status: 'REQUEST_STATUS_PENDING', timestamps };
  const listUrl = `${gateway}/v1/verification/my-requests?*`;
  function openVerification(requests) {
    cy.intercept('GET', listUrl, { requests, pagination: {} }).as('list');
    login('ROLE_EMPLOYER');
    open('Verification');
    cy.wait('@list');
  }

  it('sends a trimmed request and then shows it as pending without the form', () => {
    openVerification([]);
    cy.contains('No requests on this page.').should('be.visible');
    cy.contains('button', 'Send request').should('be.disabled');
    field('Registration number').type('  REG-123  ');
    field('Notes').type('Trade licence');
    cy.intercept('POST', `${gateway}/v1/verification/requests`, req => {
      expect(req.body).to.deep.eq({ registrationNumber: 'REG-123', notes: 'Trade licence' });
      req.reply({ request: pending });
    }).as('submit');
    cy.intercept('GET', listUrl, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply({ requests: [pending], pagination: {} });
    }).as('relist');
    cy.contains('button', 'Send request').click();
    cy.wait(['@submit', '@relist']);
    cy.contains('Request sent').should('be.visible');
    cy.contains('Your latest request is pending').should('be.visible');
    card('REG-123').should('contain', 'Pending').and('not.contain', 'by admin');
    cy.contains('button', 'Send request').should('not.exist');
  });

  it('offers a new request after a rejection and shows the reason', () => {
    openVerification([{ ...pending, status: 'REQUEST_STATUS_REJECTED', rejectionReason: 'Licence number not found', reviewedBy: adminId }]);
    card('REG-123').should('contain', 'Rejected').and('contain', 'Reason: Licence number not found').and('not.contain', adminId);
    cy.contains('Send a new request').should('be.visible');
    field('Registration number').should('have.value', '');
  });

  it('shows an approved company without the form', () => {
    openVerification([{ ...pending, status: 'REQUEST_STATUS_APPROVED', reviewedBy: adminId }]);
    cy.contains('Your company is verified').should('be.visible');
    cy.contains('button', 'Send request').should('not.exist');
  });

  it('blocks a 51 character registration number and 1001 character notes, and allows 50 and 1000', () => {
    openVerification([]);
    fill('Registration number', 'x'.repeat(51));
    cy.get('[role=alert]').should('contain', 'at most 50 characters');
    cy.contains('button', 'Send request').should('be.disabled');
    fill('Registration number', 'x'.repeat(50));
    fill('Notes', 'x'.repeat(1001));
    cy.get('[role=alert]').should('contain', 'at most 1000 characters');
    cy.contains('button', 'Send request').should('be.disabled');
    fill('Notes', 'x'.repeat(1000));
    cy.get('[role=alert]').should('not.exist');
    cy.contains('button', 'Send request').should('be.enabled');
  });

  for (const [statusCode, code, message] of [
    [409, 6, 'already have a pending request'], [400, 9, 'already verified'], [400, 3, 'was not accepted'], [403, 7, 'Only employers'],
  ]) {
    it(`explains a ${statusCode} answer with code ${code} to Send request`, () => {
      openVerification([]);
      cy.intercept('POST', `${gateway}/v1/verification/requests`, { statusCode, body: { code, message: 'private-input' } });
      field('Registration number').type('REG-123');
      cy.contains('button', 'Send request').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      cy.contains('button', 'Send request').should('be.enabled');
    });
  }
});

describe('JT-88 admin verification', () => {
  const pending = { requestId, employerId, registrationNumber: 'REG-123', notes: 'Trade licence', status: 'REQUEST_STATUS_PENDING', timestamps };
  const company = { employerId, companyName: 'Druk Tech', website: 'https://druktech.example', location: 'Thimphu', verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED' };
  const listUrl = `${gateway}/v1/verification/requests?*`;
  const approveUrl = `${gateway}/v1/verification/requests/${requestId}/approve`;
  const rejectUrl = `${gateway}/v1/verification/requests/${requestId}/reject`;
  function openQueue(response = { requests: [pending], pagination: {} }) {
    cy.intercept('GET', listUrl, response).as('list');
    login('ROLE_ADMIN');
    open('Verification');
    cy.wait('@list');
  }

  it('lists pending requests by default, shows the company and approves after confirmation', () => {
    openQueue(req => {
      expect(req.query.status).to.eq('REQUEST_STATUS_PENDING');
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply({ requests: [pending], pagination: {} });
    });
    cy.intercept('GET', `${gateway}/v1/companies/${employerId}`, { profile: company }).as('company');
    card('REG-123').contains('button', 'View company').click();
    cy.wait('@company');
    card('REG-123').should('contain', 'Company: Druk Tech').and('contain', 'Location: Thimphu');
    cy.intercept('POST', approveUrl, req => {
      expect(req.body).to.deep.eq({});
      req.reply({ request: { ...pending, status: 'REQUEST_STATUS_APPROVED', reviewedBy: adminId } });
    }).as('approve');
    card('REG-123').contains('button', 'Approve').click();
    cy.contains('Approve this verification request?').should('be.visible');
    cy.contains('button', 'Cancel').click();
    cy.get('@approve.all').should('have.length', 0);
    card('REG-123').contains('button', 'Approve').click();
    cy.intercept('GET', listUrl, { requests: [], pagination: {} }).as('relist');
    cy.contains('button', 'Confirm approval').click();
    cy.wait(['@approve', '@relist']);
    cy.contains('Request approved').should('be.visible');
    cy.contains('No requests on this page.').should('be.visible');
  });

  it('rejects with a trimmed reason of 1 to 500 characters', () => {
    openQueue();
    card('REG-123').contains('button', 'Reject').click();
    cy.contains('Reject this verification request?').should('be.visible');
    cy.contains('button', 'Confirm rejection').should('be.disabled');
    fill('Reason', 'x'.repeat(501));
    cy.get('[role=alert]').should('contain', 'at most 500 characters');
    cy.contains('button', 'Confirm rejection').should('be.disabled');
    fill('Reason', 'x'.repeat(500));
    cy.contains('button', 'Confirm rejection').should('be.enabled');
    fill('Reason', '  Licence number not found  ');
    cy.intercept('POST', rejectUrl, req => {
      expect(req.body).to.deep.eq({ reason: 'Licence number not found' });
      req.reply({ request: { ...pending, status: 'REQUEST_STATUS_REJECTED', rejectionReason: 'Licence number not found', reviewedBy: adminId } });
    }).as('reject');
    cy.contains('button', 'Confirm rejection').click();
    cy.wait('@reject');
    cy.contains('Request rejected').should('be.visible');
  });

  it('filters by status and pages through requests', () => {
    openQueue();
    const approved = { ...pending, requestId: '55555555-5555-4555-8555-555555555556', registrationNumber: 'REG-456', status: 'REQUEST_STATUS_APPROVED', reviewedBy: adminId };
    cy.intercept('GET', listUrl, req => {
      expect(req.query).not.to.have.property('status');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { requests: [approved], pagination: {} }
        : { requests: [pending], pagination: { nextPageToken: '20' } });
    }).as('all');
    choose('Status', 'All');
    cy.wait('@all');
    cy.contains('button', 'Next page').click();
    card('REG-456').should('contain', 'Approved').and('contain', `by admin ${adminId}`);
    card('REG-456').contains('button', 'Approve').should('not.exist');
  });

  for (const [statusCode, code, message] of [[400, 9, 'already been decided'], [404, 5, 'was not found'], [403, 7, 'Only admins']]) {
    it(`explains a ${statusCode} answer to an approval`, () => {
      openQueue();
      cy.intercept('POST', approveUrl, { statusCode, body: { code, message: 'private-input' } });
      card('REG-123').contains('button', 'Approve').click();
      cy.contains('button', 'Confirm approval').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      cy.contains('button', 'Confirm approval').should('be.enabled');
    });
  }

  it('says when the employer has no company profile', () => {
    openQueue();
    cy.intercept('GET', `${gateway}/v1/companies/${employerId}`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    card('REG-123').contains('button', 'View company').click();
    card('REG-123').should('contain', 'has not created a company profile').and('not.contain', 'private-input');
  });
});

describe('JT-88 verification path', () => {
  it('takes an employer from not verified to verified through an admin approval', () => {
    let profile = { employerId, companyName: 'Druk Tech', verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED', timestamps };
    let requests = [];
    cy.intercept('GET', `${gateway}/v1/employer/profile`, req => req.reply({ profile }));
    cy.intercept('GET', `${gateway}/v1/verification/my-requests?*`, req => req.reply({ requests, pagination: {} }));
    cy.intercept('POST', `${gateway}/v1/verification/requests`, req => {
      requests = [{ requestId, employerId, ...req.body, status: 'REQUEST_STATUS_PENDING', timestamps }];
      req.reply({ request: requests[0] });
    }).as('submit');
    cy.intercept('GET', `${gateway}/v1/verification/requests?*`, req =>
      req.reply({ requests: requests.filter(r => r.status === 'REQUEST_STATUS_PENDING'), pagination: {} }));
    cy.intercept('POST', `${gateway}/v1/verification/requests/${requestId}/approve`, req => {
      requests = [{ ...requests[0], status: 'REQUEST_STATUS_APPROVED', reviewedBy: adminId }];
      // In Compose, employer.verified updates the profile; the stub does it at once.
      profile = { ...profile, verificationStatus: 'VERIFICATION_STATUS_VERIFIED' };
      req.reply({ request: requests[0] });
    }).as('approve');

    login('ROLE_EMPLOYER');
    open('Company');
    cy.contains('.MuiChip-root', 'Not verified').should('be.visible');
    cy.contains('a', 'Request verification').click();
    cy.location('pathname').should('eq', '/employer/verification');
    field('Registration number').type('REG-123');
    cy.contains('button', 'Send request').click();
    cy.wait('@submit');
    cy.contains('Your latest request is pending').should('be.visible');

    cy.contains('button', 'Log out').click();
    login('ROLE_ADMIN');
    open('Verification');
    card('REG-123').contains('button', 'Approve').click();
    cy.contains('button', 'Confirm approval').click();
    cy.wait('@approve');
    cy.contains('No requests on this page.').should('be.visible');

    cy.contains('button', 'Log out').click();
    login('ROLE_EMPLOYER');
    open('Company');
    cy.contains('.MuiChip-root', exact('Verified')).should('be.visible');
    cy.contains('a', 'Request verification').should('not.exist');
    open('Verification');
    cy.contains('Your company is verified').should('be.visible');
  });
});

describe('JT-88 report a job', () => {
  const application = { applicationId: '44444444-4444-4444-8444-444444444444', jobId, resumeId: '33333333-3333-4333-8333-333333333333', applicantId: people.ROLE_APPLICANT.userId, employerId, withdrawn: false };
  const reportUrl = `${gateway}/v1/jobs/${jobId}/reports`;
  const report = { reportId, jobId, reason: 'Asks applicants for a fee', status: 'REPORT_STATUS_OPEN', timestamps };

  it('reports a job from the application card with the job ID filled in', () => {
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [application], pagination: {} }).as('applications');
    login('ROLE_APPLICANT');
    open('Applications');
    cy.wait('@applications');
    cy.contains('a', exact('Report job')).click();
    cy.location('pathname').should('eq', `/applicant/report-job/${jobId}`);
    field('Job ID').should('have.value', jobId);
    cy.contains('button', 'Send report').should('be.disabled');
    field('Reason').type('  Asks applicants for a fee  ');
    cy.intercept('POST', reportUrl, req => {
      expect(req.body).to.deep.eq({ reason: 'Asks applicants for a fee' });
      req.reply({ report });
    }).as('report');
    cy.contains('button', 'Send report').click();
    cy.wait('@report');
    cy.contains('Report sent').should('be.visible');
    field('Reason').should('have.value', '');
  });

  it('lets an employer report by ID, blocking a bad ID and a reason over 1000 characters', () => {
    login('ROLE_EMPLOYER');
    open('Report a job');
    field('Job ID').type('not-a-uuid');
    field('Reason').type('Fake job');
    cy.get('[role=alert]').should('contain', 'must be a UUID');
    cy.contains('button', 'Send report').should('be.disabled');
    field('Job ID').clear().type(jobId.toUpperCase());
    cy.contains('button', 'Send report').should('be.enabled');
    fill('Reason', 'x'.repeat(1001));
    cy.get('[role=alert]').should('contain', 'at most 1000 characters');
    cy.contains('button', 'Send report').should('be.disabled');
    fill('Reason', 'x'.repeat(1000));
    // The intercept URL is lowercase, so this also checks the ID is sent lowercase.
    cy.intercept('POST', reportUrl, req => {
      expect(req.body.reason).to.have.length(1000);
      req.reply({ report });
    }).as('report');
    cy.contains('button', 'Send report').click();
    cy.wait('@report');
  });

  it('warns about a bad job ID in the address and lets admins report too', () => {
    login('ROLE_ADMIN');
    visit('/admin/report-job/not-a-uuid');
    cy.contains('The job ID in the address is not valid').should('be.visible');
    field('Job ID').should('have.value', '');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already have an open report'], [400, 9, 'already been removed'], [400, 3, 'not accepted']]) {
    it(`explains a ${statusCode} answer with code ${code} to Send report`, () => {
      cy.intercept('POST', reportUrl, { statusCode, body: { code, message: 'private-input' } });
      login('ROLE_APPLICANT');
      open('Report a job');
      field('Job ID').type(jobId);
      field('Reason').type('Fake job');
      cy.contains('button', 'Send report').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      cy.contains('button', 'Send report').should('be.enabled');
    });
  }
});

describe('JT-88 admin moderation', () => {
  const report = { reportId, jobId, reporterId: people.ROLE_APPLICANT.userId, reason: 'Asks applicants for a fee', status: 'REPORT_STATUS_OPEN', timestamps };
  const listUrl = `${gateway}/v1/moderation/reports?*`;
  const removeUrl = `${gateway}/v1/moderation/jobs/${jobId}/remove`;
  function openReports(response = { reports: [report], pagination: {} }) {
    cy.intercept('GET', listUrl, response).as('list');
    login('ROLE_ADMIN');
    open('Moderation');
    cy.wait('@list');
  }

  it('lists open reports by default and dismisses one after confirmation', () => {
    openReports(req => {
      expect(req.query.status).to.eq('REPORT_STATUS_OPEN');
      req.reply({ reports: [report], pagination: {} });
    });
    card('Asks applicants').should('contain', 'Open').and('contain', jobId);
    cy.intercept('POST', `${gateway}/v1/moderation/reports/${reportId}/dismiss`, req => {
      expect(req.body).to.deep.eq({});
      req.reply({ report: { ...report, status: 'REPORT_STATUS_DISMISSED' } });
    }).as('dismiss');
    card('Asks applicants').contains('button', 'Dismiss').click();
    cy.contains('Dismiss this report?').should('be.visible');
    cy.contains('button', 'Cancel').click();
    cy.get('@dismiss.all').should('have.length', 0);
    card('Asks applicants').contains('button', 'Dismiss').click();
    cy.intercept('GET', listUrl, { reports: [], pagination: {} }).as('relist');
    cy.contains('button', 'Confirm dismissal').click();
    cy.wait(['@dismiss', '@relist']);
    cy.contains('Report dismissed').should('be.visible');
  });

  it('removes a reported job with a trimmed reason of 1 to 500 characters', () => {
    openReports();
    card('Asks applicants').contains('button', 'Remove job').click();
    field('Job ID').should('have.value', jobId).and('be.disabled');
    cy.contains('button', 'Confirm removal').should('be.disabled');
    fill('Reason', 'x'.repeat(501));
    cy.get('[role=alert]').should('contain', 'at most 500 characters');
    cy.contains('button', 'Confirm removal').should('be.disabled');
    fill('Reason', 'x'.repeat(500));
    cy.contains('button', 'Confirm removal').should('be.enabled');
    fill('Reason', '  Fee scam  ');
    cy.intercept('POST', removeUrl, req => {
      expect(req.body).to.deep.eq({ reason: 'Fee scam' });
      req.reply({ jobId });
    }).as('remove');
    cy.intercept('GET', listUrl, { reports: [], pagination: {} }).as('relist');
    cy.contains('button', 'Confirm removal').click();
    cy.wait(['@remove', '@relist']);
    cy.contains('Job removed').should('be.visible');
  });

  it('removes a job by ID that has no reports', () => {
    openReports({ reports: [], pagination: {} });
    cy.contains('button', 'Remove a job by ID').click();
    field('Job ID').type(jobId);
    field('Reason').type('Duplicate posting');
    cy.intercept('POST', removeUrl, req => {
      expect(req.body).to.deep.eq({ reason: 'Duplicate posting' });
      req.reply({ jobId });
    }).as('remove');
    cy.contains('button', 'Confirm removal').click();
    cy.wait('@remove');
    cy.contains('Job removed').should('be.visible');
  });

  it('filters actioned reports, which have no actions', () => {
    openReports();
    cy.intercept('GET', listUrl, req => {
      expect(req.query.status).to.eq('REPORT_STATUS_ACTIONED');
      req.reply({ reports: [{ ...report, status: 'REPORT_STATUS_ACTIONED' }], pagination: {} });
    }).as('actioned');
    choose('Status', 'Actioned');
    cy.wait('@actioned');
    card('Asks applicants').should('contain', 'Actioned');
    card('Asks applicants').contains('button', 'Dismiss').should('not.exist');
  });

  for (const [statusCode, code, message] of [[400, 9, 'already been removed'], [400, 3, 'was not removed'], [403, 7, 'Only admins']]) {
    it(`explains a ${statusCode} answer with code ${code} to a removal`, () => {
      openReports();
      cy.intercept('POST', removeUrl, { statusCode, body: { code, message: 'private-input' } });
      card('Asks applicants').contains('button', 'Remove job').click();
      field('Reason').type('Fee scam');
      cy.contains('button', 'Confirm removal').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      cy.contains('button', 'Confirm removal').should('be.enabled');
    });
  }

  it('explains a dismissal of a report that is no longer open', () => {
    openReports();
    cy.intercept('POST', `${gateway}/v1/moderation/reports/${reportId}/dismiss`, { statusCode: 400, body: { code: 9, message: 'private-input' } });
    card('Asks applicants').contains('button', 'Dismiss').click();
    cy.contains('button', 'Confirm dismissal').click();
    cy.get('[role=alert]').should('contain', 'no longer open').and('not.contain', 'private-input');
  });
});

describe('JT-88 analytics', () => {
  const summaryUrl = `${gateway}/v1/analytics/summary?*`;
  function openAnalytics() {
    login('ROLE_ADMIN');
    open('Analytics');
  }

  it('asks from the start of From to the end of To in local time and shows the counts', () => {
    const start = new Date(2026, 10, 1).toISOString();
    const end = new Date(2026, 10, 8).toISOString();
    openAnalytics();
    field('From').clear().type('2026-11-01');
    field('To').clear().type('2026-11-07');
    cy.intercept('GET', summaryUrl, req => {
      expect(req.query.start_time).to.eq(start);
      expect(req.query.end_time).to.eq(end);
      req.reply({ summary: {
        startTime: start, endTime: end, jobsPublished: '3', applicationsSubmitted: '12', offersMade: '2', offersAccepted: '1',
        statusChanges: [{ status: 'SHORTLISTED', count: '4' }, { status: 'UNDER_REVIEW', count: '5' }],
      } });
    }).as('summary');
    cy.contains('button', 'Show summary').click();
    cy.wait('@summary');
    cy.contains('table[aria-label="Outcome counts"] tr', 'Jobs published').should('contain', '3');
    cy.contains('table[aria-label="Outcome counts"] tr', 'Applications submitted').should('contain', '12');
    // protojson leaves out zero counts.
    cy.contains('table[aria-label="Outcome counts"] tr', 'Offers declined').should('contain', '0');
    cy.contains('table[aria-label="Status changes"] tr', 'Under review').should('contain', '5');
    cy.contains('table[aria-label="Status changes"] tr', 'Shortlisted').should('contain', '4');
  });

  it('blocks a To date before the From date and allows a single day', () => {
    openAnalytics();
    field('From').clear().type('2026-11-07');
    field('To').clear().type('2026-11-06');
    cy.get('[role=alert]').should('contain', 'on or after the From date');
    cy.contains('button', 'Show summary').should('be.disabled');
    field('To').clear().type('2026-11-07');
    cy.get('[role=alert]').should('not.exist');
    cy.contains('button', 'Show summary').should('be.enabled');
  });

  it('says when there are no status changes', () => {
    openAnalytics();
    cy.intercept('GET', summaryUrl, { summary: { startTime: '2026-11-01T00:00:00Z', endTime: '2026-11-08T00:00:00Z' } });
    cy.contains('button', 'Show summary').click();
    cy.contains('No status changes in this period.').should('be.visible');
    cy.contains('table[aria-label="Outcome counts"] tr', 'Jobs published').should('contain', '0');
  });

  for (const [statusCode, code, message] of [[400, 3, 'period was not accepted'], [403, 7, 'Only admins']]) {
    it(`shows a fixed message for a ${statusCode} answer`, () => {
      openAnalytics();
      cy.intercept('GET', summaryUrl, { statusCode, body: { code, message: 'private-input' } });
      cy.contains('button', 'Show summary').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
    });
  }
});

describe('JT-88 audit log', () => {
  const entry = { eventId: '88888888-8888-4888-8888-888888888888', eventType: 'employer.verified', producer: 'company-verification', occurredAt: '2026-11-16T04:00:00Z', dataJson: '{"employerId":"66666666-6666-4666-8666-666666666666"}' };
  const listUrl = `${gateway}/v1/audit/entries?*`;
  function openAudit(response = { entries: [entry], pagination: {} }) {
    cy.intercept('GET', listUrl, response).as('list');
    login('ROLE_ADMIN');
    open('Audit log');
    cy.wait('@list');
  }

  it('lists entries with the data JSON as received and pages through them', () => {
    const next = { ...entry, eventId: '88888888-8888-4888-8888-888888888889', eventType: 'job.published', producer: 'job-posting', dataJson: '{"jobId":"x"}' };
    openAudit(req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      expect(req.query).not.to.have.property('event_type');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { entries: [next], pagination: { totalCount: 2 } }
        : { entries: [entry], pagination: { nextPageToken: '20', totalCount: 2 } });
    });
    cy.contains('Matching entries: 2').should('be.visible');
    card('employer.verified').should('contain', 'company-verification').find('pre').should('have.text', entry.dataJson);
    cy.contains('button', 'Next page').click();
    card('job.published').should('contain', 'job-posting');
  });

  it('sends the event type and the local time range as filters, then clears them', () => {
    openAudit();
    field('Event type').type('application.submitted');
    field('From').type('2026-11-01T08:00');
    field('To').type('2026-11-02T08:00');
    cy.intercept('GET', listUrl, req => {
      expect(req.query.event_type).to.eq('application.submitted');
      expect(req.query.start_time).to.eq(new Date('2026-11-01T08:00').toISOString());
      expect(req.query.end_time).to.eq(new Date('2026-11-02T08:00').toISOString());
      expect(req.query['pagination.page_token']).to.eq('');
      req.reply({ entries: [], pagination: {} });
    }).as('filtered');
    cy.contains('button', 'Apply filters').click();
    cy.wait('@filtered');
    cy.contains('No entries on this page.').should('be.visible');
    cy.intercept('GET', listUrl, req => {
      expect(req.query).not.to.have.property('event_type');
      expect(req.query).not.to.have.property('start_time');
      req.reply({ entries: [entry], pagination: {} });
    }).as('cleared');
    cy.contains('button', 'Clear filters').click();
    cy.wait('@cleared');
    field('Event type').should('have.value', '');
  });

  it('blocks an event type that is not lowercase with dots and a From time that is not before To', () => {
    openAudit();
    field('Event type').type('Application.Submitted');
    cy.get('[role=alert]').should('contain', 'lowercase words joined by dots');
    cy.contains('button', 'Apply filters').should('be.disabled');
    field('Event type').clear().type('job_alert.matched');
    field('From').type('2026-11-02T08:00');
    field('To').type('2026-11-02T08:00');
    cy.get('[role=alert]').should('contain', 'From time must be before');
    cy.contains('button', 'Apply filters').should('be.disabled');
  });

  it('shows a fixed message when the filters are rejected', () => {
    openAudit();
    cy.intercept('GET', listUrl, { statusCode: 400, body: { code: 3, message: 'private-input' } });
    field('Event type').type('application.submitted');
    cy.contains('button', 'Apply filters').click();
    cy.get('[role=alert]').should('contain', 'filters were not accepted').and('not.contain', 'private-input');
  });
});

describe('JT-88 role-limited routes', () => {
  const adminLinks = ['Moderation', 'Analytics', 'Audit log'];

  it('gives applicants no employer or admin links or routes', () => {
    login('ROLE_APPLICANT');
    cy.contains('nav a', exact('Report a job')).should('be.visible');
    for (const link of ['Company', 'Verification', ...adminLinks]) cy.contains('nav a', exact(link)).should('not.exist');
    visit('/admin/moderation');
    cy.location('pathname').should('eq', '/applicant');
    visit('/employer/company');
    cy.location('pathname').should('eq', '/applicant');
  });

  it('gives employers Company and Verification but no admin pages', () => {
    login('ROLE_EMPLOYER');
    for (const link of ['Company', 'Verification', 'Report a job']) cy.contains('nav a', exact(link)).should('be.visible');
    for (const link of adminLinks) cy.contains('nav a', exact(link)).should('not.exist');
    visit('/admin/audit-log');
    cy.location('pathname').should('eq', '/employer');
  });

  it('gives admins the admin pages but no applicant or employer pages', () => {
    login('ROLE_ADMIN');
    for (const link of ['Verification', ...adminLinks, 'Report a job']) cy.contains('nav a', exact(link)).should('be.visible');
    for (const link of ['Applications', 'Company']) cy.contains('nav a', exact(link)).should('not.exist');
    visit('/employer/verification');
    cy.location('pathname').should('eq', '/admin');
    visit('/admin/unknown');
    cy.location('pathname').should('eq', '/admin');
    cy.contains('h1', 'Admin workspace').should('be.visible');
  });
});
