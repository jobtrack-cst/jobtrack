const gateway = 'http://localhost:8000';
// Keep the applicant app-bar photo request independent of a running gateway.
beforeEach(() => cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} }));
beforeEach(() => {
  cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: '30000000-0000-4000-8000-000000000001', name: 'Thimphu' }] });
});
function locationSelect() {
  return cy.contains('label', /^Location$/).parent().find('[role="combobox"]');
}
function selectLocation(name) {
  // MUI's non-native select is a div; Cypress does not wait on aria-disabled.
  locationSelect().should('be.visible').and('not.have.attr', 'aria-disabled', 'true').click();
  locationSelect().should('have.attr', 'aria-expanded', 'true');
  cy.get('[role="listbox"]').should('be.visible')
    .contains('[role="option"]', new RegExp(`^${Cypress._.escapeRegExp(name)}$`)).click();
  locationSelect().should('have.attr', 'aria-expanded', 'false');
}
const applicantId = '11111111-1111-4111-8111-111111111111';
const jobId = '22222222-2222-4222-8222-222222222222';
const resumeId = '33333333-3333-4333-8333-333333333333';
const applicationId = '44444444-4444-4444-8444-444444444444';
const offerId = '55555555-5555-4555-8555-555555555555';
const application = { applicationId, jobId, resumeId, applicantId, employerId: '66666666-6666-4666-8666-666666666666', withdrawn: false };
const offer = { offerId, applicationId, terms: 'Synthetic offer terms', status: 'OFFER_STATUS_PENDING' };
function login(role = 'ROLE_APPLICANT') {
  const user = { userId: applicantId, email: 'pema@example.test', fullName: 'Pema', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'workflow-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every protected workflow request must pass through the shared gateway client.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer workflow-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', role === 'ROLE_APPLICANT' ? '/applicant' : role === 'ROLE_EMPLOYER' ? '/employer' : '/admin');
}
function navigate(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
function setupApply(status = 'JOB_STATUS_PUBLISHED') {
  cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [
    { resumeId, fileName: 'validated.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_VALIDATED' },
    { resumeId: 'pending', fileName: 'pending.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_PENDING' },
    { resumeId: 'rejected', fileName: 'rejected.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_REJECTED' },
  ] }).as('resumes');
  cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { jobId, title: 'Synthetic internship', description: 'Job description', status, deadline: new Date(Date.now() + 86400000).toISOString() } }).as('job');
  login();
  cy.contains('a', 'Apply').click();
  cy.get('input').first().type(jobId);
  cy.contains('button', 'Load job').click();
  cy.wait(['@resumes', '@job']);
  cy.get('[role=combobox]').click();
  cy.get('[role=option]').should('have.length', 1).and('contain', 'validated.pdf').click();
}
function applications(role = 'ROLE_APPLICANT') {
  cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [application], pagination: {} }).as('applications');
  login(role);
  cy.contains('a', 'Applications').click();
  cy.wait('@applications');
}
describe('JT-78 apply and applications', () => {
  it('applies with only a validated resume, then lists and withdraws the application', () => {
    cy.intercept('POST', `${gateway}/v1/applications`, req => {
      expect(req.body).to.deep.eq({ jobId, resumeId });
      req.reply({ application });
    }).as('submit');
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [application] });
    setupApply();
    cy.contains('button', 'Submit application').click();
    cy.wait('@submit');
    cy.contains('Application submitted.').should('be.visible');
    cy.contains('button', 'Submit application').should('be.disabled');
    cy.contains('a', 'My applications').click();
    cy.contains(applicationId).should('be.visible');
    cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/withdraw`, req => {
      expect(req.body).to.deep.eq({});
      req.reply({ application: { ...application, withdrawn: true } });
    }).as('withdraw');
    cy.contains('button', 'Withdraw').click();
    cy.contains('button', 'Cancel').click();
    cy.contains('button', 'Withdraw').click();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [{ ...application, withdrawn: true }] });
    cy.contains('button', 'Confirm withdrawal').click();
    cy.wait('@withdraw');
    cy.contains('Application withdrawn.').should('be.visible');
    cy.contains('button', 'Withdraw').should('be.disabled');
  });
  it('blocks a closed job before submission', () => {
    setupApply('JOB_STATUS_CLOSED');
    cy.contains('This job is closed').should('be.visible');
    cy.contains('button', 'Submit application').should('be.disabled');
  });
  for (const [statusCode, code, expected] of [[400, 9, 'resume is not validated'], [409, 6, 'already applied'], [403, 7, 'permission'], [404, 5, 'not found']]) {
    it(`shows safe business error ${code} and permits retry`, () => {
      cy.intercept('POST', `${gateway}/v1/applications`, { statusCode, body: { code, message: 'private-input' } });
      setupApply();
      cy.contains('button', 'Submit application').click();
      cy.get('[role=alert]').should('contain', expected).and('not.contain', 'private-input');
      cy.contains('button', 'Submit application').should('be.enabled');
    });
  }
  it('handles empty and failed resume lists with refresh', () => {
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { statusCode: 503, body: {} });
    login(); navigate('/applicant/apply');
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [] });
    cy.contains('button', 'Refresh resumes').click();
    cy.contains('No validated resume').should('be.visible');
    cy.contains('button', 'Submit application').should('be.disabled');
  });
  it('uses nested pagination query fields and reaches later applications', () => {
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { applications: [], pagination: {} }
        : { applications: [application], pagination: { nextPageToken: '20' } });
    });
    login(); cy.contains('a', 'Applications').click();
    cy.contains(applicationId).should('be.visible');
    cy.contains('button', 'Next page').click();
    cy.contains('No applications').should('be.visible');
    cy.contains('button', 'First page').click();
    cy.contains(applicationId).should('be.visible');
  });
});
describe('JT-78 offers', () => {
  it('makes an employer offer from an application without client participant IDs', () => {
    applications('ROLE_EMPLOYER');
    cy.contains('a', 'Make offer').click();
    cy.get('input').should('have.value', applicationId);
    cy.get('textarea').first().type('Synthetic offer terms');
    cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/offers`, req => {
      expect(req.body).to.deep.eq({ terms: 'Synthetic offer terms' }); req.reply({ offer });
    }).as('make');
    cy.contains('button', 'Make offer').click(); cy.wait('@make');
    cy.contains(offerId).should('be.visible');
    cy.contains('button', 'Make offer').should('be.disabled');
    cy.contains('button', 'Accept offer').should('not.exist');
  });
  for (const decision of ['accept', 'decline']) {
    it(`views and confirms ${decision}; final offers cannot be answered again`, () => {
      cy.intercept('GET', `${gateway}/v1/offers/${offerId}`, { offer });
      cy.intercept('POST', `${gateway}/v1/offers/${offerId}/${decision}`, req => {
        expect(req.body).to.deep.eq({});
        req.reply({ offer: { ...offer, status: decision === 'accept' ? 'OFFER_STATUS_ACCEPTED' : 'OFFER_STATUS_DECLINED' } });
      }).as('decision');
      login(); cy.contains('a', 'Offers').click();
      cy.get('input').type(offerId); cy.contains('button', 'Load offer').click();
      cy.contains('Synthetic offer terms').should('be.visible');
      cy.contains('button', decision === 'accept' ? 'Accept offer' : 'Decline offer').click();
      cy.contains('button', 'Confirm decision').click(); cy.wait('@decision');
      cy.contains('Status: ' + (decision === 'accept' ? 'ACCEPTED' : 'DECLINED')).should('be.visible');
      cy.contains('button', 'Accept offer').should('not.exist');
      cy.contains('button', 'Decline offer').should('not.exist');
    });
  }
});
describe('JT-78 messages and alerts', () => {
  for (const role of ['ROLE_APPLICANT', 'ROLE_EMPLOYER']) {
    it(`reads and sends application messages for ${role}`, () => {
      cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/messages?*`, { messages: [{ messageId: 'm1', senderId: 'other', content: 'Existing message' }] });
      applications(role); cy.contains('a', 'Messages').click();
      cy.contains('Existing message').should('be.visible');
      cy.get('textarea').first().type('  Reply  ');
      cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/messages`, req => {
        expect(req.body).to.deep.eq({ content: 'Reply' }); req.reply({ message: { messageId: 'm2' } });
      }).as('send');
      cy.contains('button', 'Send message').click(); cy.wait('@send');
      cy.contains('Message sent.').should('be.visible');
      cy.get('textarea').first().should('have.value', '');
    });
  }
  it('creates and deletes an exact-location subscription; empty filters cannot submit', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    login(); cy.contains('a', 'Job alerts').click();
    cy.contains('button', 'Create subscription').should('be.disabled');
    selectLocation('Thimphu');
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [], tagIds: [], location: 'Thimphu' });
      req.reply({ jobAlert: { alertId: offerId, location: 'Thimphu' } });
    }).as('create');
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [{ alertId: offerId, location: 'Thimphu' }] });
    cy.contains('button', 'Create subscription').click(); cy.wait('@create');
    cy.contains('Subscription created.').should('be.visible');
    cy.contains('Location: Thimphu').should('be.visible');
    cy.intercept('DELETE', `${gateway}/v1/job-alerts/${offerId}`, { body: {} }).as('delete');
    cy.contains('button', 'Delete subscription').click();
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.contains('button', 'Confirm deletion').click(); cy.wait('@delete');
    cy.contains('No subscriptions').should('be.visible');
  });
  it('restricts feature routes by role', () => {
    login('ROLE_EMPLOYER'); navigate('/applicant/job-alerts');
    cy.location('pathname').should('eq', '/employer');
    navigate('/applicant/apply'); cy.location('pathname').should('eq', '/employer');
  });
  it('redirects expired workflow sessions to login on protected 401', () => {
    cy.intercept('GET', `${gateway}/v1/applications?*`, { statusCode: 401, body: { code: 16 } });
    login(); cy.contains('a', 'Applications').click();
    cy.location('pathname').should('eq', '/login');
  });
});

describe('JT-78 boundaries and recovery', () => {
  it('loads validated resumes on a later page', () => {
    cy.intercept('GET', `${gateway}/v1/resumes?*`, req => {
      req.reply(req.query['pagination.page_token'] === '20'
        ? { resumes: [{ resumeId, fileName: 'later.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_VALIDATED' }] }
        : { resumes: [], pagination: { nextPageToken: '20' } });
    });
    login(); navigate('/applicant/apply');
    cy.contains('No validated resume').should('be.visible');
    cy.contains('button', 'Next page').click();
    cy.get('[role=combobox]').click(); cy.get('[role=option]').contains('later.pdf').click();
  });
  it('blocks a passed deadline even when the job is published', () => {
    setupApply();
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { jobId, title: 'Expired', status: 'JOB_STATUS_PUBLISHED', deadline: new Date(Date.now() - 1000).toISOString() } });
    cy.contains('button', 'Load job').click();
    cy.contains('This job is closed').should('be.visible');
    cy.contains('button', 'Submit application').should('be.disabled');
  });
  it('prevents duplicate in-flight application submissions', () => {
    cy.intercept('POST', `${gateway}/v1/applications`, { delay: 800, body: { application } }).as('submit');
    setupApply();
    cy.contains('button', 'Submit application').click().should('be.disabled');
    cy.wait('@submit');
    cy.get('@submit.all').should('have.length', 1);
  });
  it('bounds offer terms at 2000 characters and reports duplicate offers', () => {
    applications('ROLE_EMPLOYER'); cy.contains('a', 'Make offer').click();
    cy.contains('button', 'Make offer').should('be.disabled');
    // Use native value setter to notify React, as a user edit would.
    cy.get('textarea').first().then($el => {
      const el = $el[0];
      Object.getOwnPropertyDescriptor(el.ownerDocument.defaultView.HTMLTextAreaElement.prototype, 'value').set.call(el, 'x'.repeat(2001));
      el.dispatchEvent(new el.ownerDocument.defaultView.Event('input', { bubbles: true }));
    });
    cy.contains('button', 'Make offer').should('be.disabled');
    cy.get('textarea').first().clear().type('Terms');
    cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/offers`, { statusCode: 409, body: { code: 6 } });
    cy.contains('button', 'Make offer').click();
    cy.contains('Only one lifetime offer').should('be.visible');
  });
  it('keeps the thread draft on failure and permits retry', () => {
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/messages?*`, { messages: [] });
    applications(); cy.contains('a', 'Messages').click();
    cy.contains('No messages').should('be.visible');
    cy.contains('button', 'Send message').should('be.disabled');
    cy.get('textarea').first().type('Retry this draft');
    cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/messages`, { statusCode: 503, body: {} });
    cy.contains('button', 'Send message').click();
    cy.contains('The request failed').should('be.visible');
    cy.get('textarea').first().should('have.value', 'Retry this draft');
    cy.contains('button', 'Send message').should('be.enabled');
  });
  it('shows permission errors for a non-participant thread and blocks sending', () => {
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/messages?*`, { statusCode: 403, body: { code: 7 } });
    applications(); cy.contains('a', 'Messages').click();
    cy.contains('You do not have permission').should('be.visible');
    cy.get('textarea').first().type('Cannot send');
    cy.contains('button', 'Send message').should('be.disabled');
  });
  for (const [statusCode, code, text] of [[409, 6, 'subscription already exists'], [400, 9, 'at most 10 subscriptions']]) {
    it(`reports alert constraint ${code}`, () => {
      cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
      cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
      cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
      cy.intercept('POST', `${gateway}/v1/job-alerts`, { statusCode, body: { code } });
      login(); cy.contains('a', 'Job alerts').click();
      selectLocation('Thimphu'); cy.contains('button', 'Create subscription').click();
      cy.get('[role=alert]').should('contain', text);
    });
  }
  it('paginates reference data and submits Any as blank with category/tag filters', () => {
    const categoryId = '10000000-0000-4000-8000-000000000001';
    const tagId = '20000000-0000-4000-8000-000000000001';
    cy.intercept('GET', `${gateway}/v1/categories?*`, req => {
      req.reply(req.query['pagination.page_token'] === '20'
        ? { categories: [{ categoryId, name: 'Engineering' }] }
        : { categories: [], pagination: { nextPageToken: '20' } });
    });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId, name: 'Go' }] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [categoryId], tagIds: [tagId], location: '' }); req.reply({ jobAlert: {} });
    }).as('filters');
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').first().type('Engineering'); cy.get('[role=option]').contains('Engineering').click();
    cy.get('[role=combobox]').eq(1).type('Go'); cy.get('[role=option]').contains('Go').click();
    selectLocation('Thimphu');
    selectLocation('Any');
    cy.contains('button', 'Create subscription').click(); cy.wait('@filters');
  });
  for (const path of ['/applicant/apply', '/applicant/offers', '/applicant/job-alerts', `/employer/offers/new/${applicationId}`, `/employer/messages/${applicationId}`]) {
    it(`protects direct unauthenticated route ${path}`, () => {
      cy.visit(path); cy.location('pathname').should('eq', '/login');
    });
  }
  it('does not expose applicant or employer workflows to admins', () => {
    login('ROLE_ADMIN'); navigate('/applicant/applications'); cy.location('pathname').should('eq', '/admin');
    navigate(`/employer/offers/new/${applicationId}`); cy.location('pathname').should('eq', '/admin');
  });
});


describe('JT-78 location reference selection', () => {
  it('waits for location references before opening the non-native select', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    let releaseLocations;
    const referencesReady = new Promise(resolve => { releaseLocations = resolve; });
    cy.intercept('GET', `${gateway}/v1/locations?*`, req => referencesReady.then(() => {
      req.reply({ locations: [{ locationId: 'slow', name: 'Thimphu' }] });
    })).as('slowLocations');
    login(); cy.contains('a', 'Job alerts').click();
    // An early click is ignored, even though Cypress permits it on this div.
    locationSelect().should('have.attr', 'aria-disabled', 'true').click();
    locationSelect().should('have.attr', 'aria-expanded', 'false');
    cy.get('[role="listbox"]').should('not.exist');
    cy.then(() => releaseLocations());
    selectLocation('Thimphu');
    cy.wait('@slowLocations');
    locationSelect().should('contain', 'Thimphu');
    cy.contains('button', 'Create subscription').should('be.enabled');
  });
  it('loads every location reference page and submits the exact returned name', () => {
    const name = 'Remote / Hybrid';
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('GET', `${gateway}/v1/locations?*`, req => {
      expect(req.headers.authorization).to.eq('Bearer workflow-token');
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { locations: [{ locationId: 'later', name }] }
        : { locations: [{ locationId: 'first', name: 'Thimphu' }], pagination: { nextPageToken: '20' } });
    }).as('locations');
    cy.intercept('POST', `${gateway}/v1/job-alerts`, req => {
      expect(req.body).to.deep.eq({ categoryIds: [], tagIds: [], location: name });
      req.reply({ jobAlert: {} });
    }).as('exactLocation');
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').last().should('not.have.attr', 'aria-disabled', 'true').click();
    cy.get('[role=option]').should('have.length', 3);
    cy.get('[role=option]').contains(name).click();
    cy.contains('button', 'Create subscription').click(); cy.wait('@exactLocation');
    cy.get('@locations.all').should('have.length', 2);
  });
  it('preserves reference loading/error states and supports refresh after location failure', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [] });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [] });
    cy.intercept('GET', `${gateway}/v1/job-alerts?*`, { jobAlerts: [] });
    cy.intercept('GET', `${gateway}/v1/locations?*`, { delay: 300, statusCode: 503, body: {} });
    login(); cy.contains('a', 'Job alerts').click();
    cy.get('[role=combobox]').last().should('have.attr', 'aria-disabled', 'true');
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: 'restored', name: 'Thimphu' }] });
    cy.contains('button', 'Refresh subscriptions').click();
    selectLocation('Thimphu');
    cy.contains('button', 'Create subscription').should('be.enabled');
  });
});

// JT-108: discover the lifetime offer through either participant's application.
describe('JT-108 offer discovery', () => {
  for (const role of ['ROLE_APPLICANT', 'ROLE_EMPLOYER']) {
    const base = role === 'ROLE_EMPLOYER' ? '/employer' : '/applicant';
    it(`opens, revisits and reloads the offer from application cards for ${role}`, () => {
      cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { delay: 500, body: { offer } }).as('byApplication');
      applications(role);
      cy.contains('a', 'View Offer').should('have.attr', 'href', `${base}/offers?applicationId=${applicationId}`).click();
      cy.contains('Loading offer').should('be.visible');
      cy.wait('@byApplication');
      cy.contains(offer.terms).should('be.visible');
      cy.contains('button', 'Accept offer').should(role === 'ROLE_APPLICANT' ? 'exist' : 'not.exist');
      cy.contains('a', 'Applications').click();
      cy.contains('a', 'View Offer').click(); cy.wait('@byApplication');
      cy.contains(offerId).should('be.visible');
      // JT-106 revalidates the session and preserves the offer deep link.
      cy.reload(); cy.location('pathname').should('eq', `${base}/offers`);
      cy.location('search').should('eq', `?applicationId=${applicationId}`);
      cy.wait('@byApplication');
      cy.contains(offer.terms).should('be.visible');
    });
  }
  it('shows no offer separately from server errors and allows retry', () => {
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { statusCode: 404, body: { code: 5 } }).as('missingOffer');
    applications(); cy.contains('a', 'View Offer').click(); cy.wait('@missingOffer');
    cy.contains('No offer has been made for this application.').should('be.visible');
    cy.contains('button', 'Accept offer').should('not.exist');
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { statusCode: 503, body: { code: 14, message: 'private-details' } }).as('failedOffer');
    cy.contains('button', 'Load offer').click(); cy.wait('@failedOffer');
    cy.get('[role=alert]').should('contain', 'The request failed').and('not.contain', 'private-details');
    cy.contains('No offer has been made').should('not.exist');
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { offer });
    cy.contains('button', 'Load offer').click();
    cy.contains(offer.terms).should('be.visible');
  });
  it('does not request a malformed application ID', () => {
    login(); navigate('/applicant/offers?applicationId=bad');
    cy.contains('Application ID must be a UUID.').should('be.visible');
    cy.contains('button', 'Load offer').should('be.disabled');
  });
  it('switches from the existing offer-ID lookup to application lookup', () => {
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { offer }).as('byApplication');
    login(); cy.contains('a', 'Offers').click();
    cy.contains('button', 'Use application ID').click();
    cy.get('input').type(applicationId);
    cy.contains('button', 'Load offer').click(); cy.wait('@byApplication');
    cy.contains(offer.terms).should('be.visible');
  });
  it('keeps a pending offer available after a failed decision for retry', () => {
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { offer });
    applications(); cy.contains('a', 'View Offer').click();
    cy.contains('button', 'Accept offer').click();
    cy.intercept('POST', `${gateway}/v1/offers/${offerId}/accept`, { statusCode: 503, body: {} }).as('failedDecision');
    cy.contains('button', 'Confirm decision').click(); cy.wait('@failedDecision');
    cy.contains('button', 'Confirm decision').should('be.enabled');
    cy.intercept('POST', `${gateway}/v1/offers/${offerId}/accept`, { offer: { ...offer, status: 'OFFER_STATUS_ACCEPTED' } });
    cy.contains('button', 'Confirm decision').click();
    cy.contains('Status: ACCEPTED').should('be.visible');
  });
  it('links a newly created offer to a reloadable application lookup', () => {
    cy.intercept('POST', `${gateway}/v1/applications/${applicationId}/offers`, { offer });
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, { offer }).as('createdOffer');
    applications('ROLE_EMPLOYER'); cy.contains('a', 'Make offer').click();
    cy.get('textarea').first().type('Synthetic offer terms');
    cy.contains('button', 'Make offer').click();
    cy.contains('a', 'View Offer').click(); cy.wait('@createdOffer');
    cy.location('search').should('eq', `?applicationId=${applicationId}`);
    cy.contains(offer.terms).should('be.visible');
  });
  it('ignores an old response when navigating to another application on the same offers page', () => {
    const otherId = '77777777-7777-4777-8777-777777777777';
    let release;
    const pending = new Promise(resolve => { release = resolve; });
    cy.intercept('GET', `${gateway}/v1/applications/${applicationId}/offer`, req => pending.then(() => req.reply({ offer }))).as('oldOffer');
    cy.intercept('GET', `${gateway}/v1/applications/${otherId}/offer`, { offer: { ...offer, applicationId: otherId, terms: 'Another application offer' } }).as('newOffer');
    applications(); cy.contains('a', 'View Offer').click();
    cy.contains('Loading offer').should('be.visible');
    navigate(`/applicant/offers?applicationId=${otherId}`); cy.wait('@newOffer');
    cy.contains('Another application offer').should('be.visible');
    cy.then(() => release()); cy.wait('@oldOffer');
    cy.contains('Another application offer').should('be.visible');
    cy.contains(offer.terms).should('not.exist');
  });
});
