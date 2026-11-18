// JT-73 gateway-stubbed UI workflows; live steps are in docs/testing/jt-73-frontend.md.
const gateway = 'http://localhost:8000';
const jobId = '22222222-2222-4222-8222-222222222222';
const categoryId = '33333333-3333-4333-8333-333333333333';
const tagId = '44444444-4444-4444-8444-444444444444';
const notificationId = '55555555-5555-4555-8555-555555555555';
const userId = '11111111-1111-4111-8111-111111111111';
const deadline = new Date(Date.now() + 7 * 86400000).toISOString();
const job = { jobId, employerId: userId, title: 'Frontend intern', description: 'Build accessible pages', categoryId, tagIds: [tagId], location: 'Thimphu', deadline, status: 'JOB_STATUS_DRAFT' };
const notice = { notificationId, title: 'Interview scheduled', body: '<script>private script</script>', createdAt: new Date().toISOString() };
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
const field = label => cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
const open = label => cy.contains('nav a', exact(label)).click();
const card = text => cy.contains('.MuiCard-root', text);
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
function choose(label, option) {
  cy.contains('label', new RegExp(`^${label}`)).parent().find('[role=combobox]').click();
  cy.contains('[role=option]', exact(option)).click();
}
function fill(label, text) {
  field(label).then($el => {
    const el = $el[0];
    const view = el.ownerDocument.defaultView;
    const proto = el.tagName === 'TEXTAREA' ? view.HTMLTextAreaElement.prototype : view.HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, text);
    el.dispatchEvent(new view.Event('input', { bubbles: true }));
  });
}
function login(role = 'ROLE_EMPLOYER') {
  const user = { userId, fullName: 'Pema', email: 'pema@example.test', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt73-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt73-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', `/${role.slice(5).toLowerCase()}`);
}
function references() {
  cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [{ categoryId, name: 'Technology' }], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId, name: 'React' }], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: categoryId, name: 'Thimphu' }], pagination: {} });
}
function manage(value = job) {
  cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: value });
  login(); visit(`/employer/jobs/${jobId}`);
  cy.contains('Status:').should('be.visible');
}
beforeEach(references);

describe('JT-73 employer jobs', () => {
  it('lists own jobs with pagination and creates, edits, publishes and closes a job', () => {
    let current = { ...job };
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20' ? { jobs: [], pagination: { totalCount: 1 } }
        : { jobs: [current], pagination: { nextPageToken: '20', totalCount: 1 } });
    });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, req => req.reply({ job: current }));
    cy.intercept('POST', `${gateway}/v1/jobs`, req => {
      expect(req.body).to.deep.eq({ title: 'Frontend intern', description: 'Build accessible pages', categoryId, tagIds: [tagId], location: 'Thimphu', deadline: new Date('2030-01-20T12:00').toISOString() });
      expect(req.body).not.to.have.property('employerId');
      current = { ...job, ...req.body }; req.reply({ job: current });
    }).as('create');
    cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, req => {
      expect(req.body.title).to.eq('Updated intern');
      expect(req.body.categoryId).to.eq(categoryId);
      expect(req.body.tagIds).to.deep.eq([tagId]);
      expect(req.body.description).to.eq(job.description);
      current = { ...current, ...req.body }; req.reply({ job: current });
    }).as('update');
    for (const [action, status] of [['publish', 'JOB_STATUS_PUBLISHED'], ['close', 'JOB_STATUS_CLOSED']]) {
      cy.intercept('POST', `${gateway}/v1/jobs/${jobId}/${action}`, req => {
        expect(req.body).to.deep.eq({}); current = { ...current, status }; req.reply({ job: current });
      }).as(action);
    }
    login(); open('My jobs');
    card(job.title).should('contain', 'Draft');
    cy.contains('button', 'Next page').click(); cy.contains('No jobs on this page.').should('be.visible');
    cy.contains('button', 'First page').click();
    cy.contains('a', exact('Create job')).click();
    field('Title').type('  Frontend intern  '); field('Description').type('  Build accessible pages  ');
    choose('Category', 'Technology'); choose('Tags', 'React'); field('Tags').type('{esc}');
    field('Location').type('Thimphu{esc}'); field('Deadline').type('2030-01-20T12:00');
    cy.contains('button', 'Create draft').click(); cy.wait('@create');
    cy.location('pathname').should('eq', `/employer/jobs/${jobId}`);
    field('Title').clear().type('Updated intern'); cy.contains('button', 'Save changes').click(); cy.wait('@update');
    cy.contains('Job updated.').should('be.visible');
    cy.contains('button', exact('Publish job')).click();
    cy.contains('button', 'Cancel').click(); cy.get('@publish.all').should('have.length', 0);
    cy.contains('button', exact('Publish job')).click(); cy.contains('button', 'Confirm publish').click(); cy.wait('@publish');
    cy.contains('Status: Published').should('be.visible');
    cy.contains('button', exact('Close job')).click(); cy.contains('button', 'Confirm close').click(); cy.wait('@close');
    cy.contains('Status: Closed').should('be.visible'); cy.contains('button', 'Save changes').should('not.exist');
    cy.contains('button', exact('Publish job')).should('not.exist');
  });

  it('enforces trimmed Unicode text limits and a future local deadline', () => {
    manage();
    fill('Title', ' '); cy.contains('button', 'Save changes').should('be.disabled');
    fill('Title', '界'.repeat(151)); cy.contains('Title must contain 1 to 150').should('be.visible');
    fill('Title', '界'.repeat(150)); cy.contains('button', 'Save changes').should('be.enabled');
    fill('Description', 'x'.repeat(5001)); cy.contains('button', 'Save changes').should('be.disabled');
    fill('Description', 'x'.repeat(5000)); cy.contains('button', 'Save changes').should('be.enabled');
    fill('Location', 'x'.repeat(101)); cy.contains('button', 'Save changes').should('be.disabled');
    fill('Location', 'x'.repeat(100)); cy.contains('button', 'Save changes').should('be.enabled');
    field('Deadline').clear().type('2020-01-01T12:00'); cy.contains('button', 'Save changes').should('be.disabled');
  });

  it('loads all reference pages and lets a published job be edited', () => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, req => req.reply(req.query['pagination.page_token'] === '20'
      ? { categories: [{ categoryId, name: 'Technology' }], pagination: {} }
      : { categories: [], pagination: { nextPageToken: '20' } }));
    manage({ ...job, status: 'JOB_STATUS_PUBLISHED' });
    cy.contains('label', /^Category/).parent().find('[role=combobox]').should('not.have.attr', 'aria-disabled', 'true');
    choose('Category', 'Technology'); cy.contains('button', 'Save changes').should('be.enabled');
    cy.contains('button', 'Close job').should('be.visible');
  });

  for (const status of ['JOB_STATUS_DRAFT', 'JOB_STATUS_PUBLISHED']) {
    it(`preserves the exact deadline on a description-only edit of ${status}`, () => {
      const when = new Date(Date.now() + 7 * 86400000);
      when.setUTCSeconds(45);
      const originalDeadline = when.toISOString().replace(/\.\d{3}Z$/, '.123456Z');
      let current = { ...job, status, deadline: originalDeadline };
      cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, req => req.reply({ job: current }));
      cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, req => {
        expect(req.body).to.deep.eq({ title: job.title, description: 'Description-only update', categoryId, tagIds: [tagId], location: job.location, deadline: originalDeadline });
        current = { ...current, ...req.body }; req.reply({ job: current });
      }).as('update');
      login(); visit(`/employer/jobs/${jobId}`);
      field('Description').clear().type('Description-only update');
      cy.contains('button', 'Save changes').click(); cy.wait('@update');
      cy.contains('Job updated.').should('be.visible');
      cy.contains('button', 'Refresh job').click();
      field('Description').should('have.value', 'Description-only update');
    });
  }

  it('converts an explicitly changed local deadline to UTC', () => {
    manage();
    cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, req => {
      expect(req.body.deadline).to.eq(new Date('2030-01-20T12:00').toISOString());
      req.reply({ job: { ...job, ...req.body } });
    }).as('update');
    field('Deadline').clear().type('2020-01-01T12:00');
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Deadline').clear().type('2030-01-20T12:00');
    cy.contains('button', 'Save changes').click(); cy.wait('@update');
  });

  for (const [action, status, message] of [
    ['publish', 'JOB_STATUS_DRAFT', 'company must be verified'],
    ['close', 'JOB_STATUS_PUBLISHED', 'no longer allowed'],
  ]) {
    it(`shows a safe ${action} failure inside the dialog and allows cancel and retry`, () => {
      let current = { ...job, status };
      let attempts = 0;
      cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, req => req.reply({ job: current }));
      cy.intercept('POST', `${gateway}/v1/jobs/${jobId}/${action}`, req => {
        attempts++;
        if (attempts <= 2) req.reply({ statusCode: 400, body: { code: 9, message: 'private-input' } });
        else {
          current = { ...current, status: action === 'publish' ? 'JOB_STATUS_PUBLISHED' : 'JOB_STATUS_CLOSED' };
          req.reply({ job: current });
        }
      }).as('transition');
      login(); visit(`/employer/jobs/${jobId}`);
      const openConfirmation = () => cy.contains('button', exact(action === 'publish' ? 'Publish job' : 'Close job')).click();
      const assertError = () => cy.get('[role=dialog]').should('be.visible').within(() => {
        cy.get('[role=alert]').should('be.visible').and('contain', message).and('not.contain', 'private-input');
        cy.get('[role=alert]').parents('[aria-hidden=true]').should('not.exist');
        cy.contains('button', `Confirm ${action}`).should('be.enabled');
        cy.contains('button', 'Cancel').should('be.enabled');
      });
      openConfirmation(); cy.get('[role=dialog]').contains('button', `Confirm ${action}`).click(); cy.wait('@transition');
      assertError();
      cy.get('[role=dialog]').contains('button', 'Cancel').click();
      cy.get('[role=dialog]').should('not.exist');
      cy.get('[role=alert]').should('not.exist');
      cy.get('@transition.all').should('have.length', 1);
      openConfirmation(); cy.get('[role=dialog]').find('[role=alert]').should('not.exist');
      cy.get('[role=dialog]').contains('button', `Confirm ${action}`).click(); cy.wait('@transition');
      assertError();
      cy.get('[role=dialog]').contains('button', `Confirm ${action}`).click(); cy.wait('@transition');
      cy.get('[role=dialog]').should('not.exist');
      cy.contains(action === 'publish' ? 'Status: Published' : 'Status: Closed').should('be.visible');
      cy.get('@transition.all').should('have.length', 3);
    });
  }

  for (const [statusCode, code, message] of [[403, 7, 'owns this job'], [400, 9, 'no longer allowed'], [500, 13, 'request failed']]) {
    it(`safely handles update failure ${code} and preserves entered fields`, () => {
      manage();
      cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, { statusCode, body: { code, message: 'private-input' } });
      field('Title').clear().type('Keep this title'); cy.contains('button', 'Save changes').click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      field('Title').should('have.value', 'Keep this title'); cy.contains('button', 'Save changes').should('be.enabled');
    });
  }

  it('shows removed jobs without editing or lifecycle actions', () => {
    manage({ ...job, status: 'JOB_STATUS_REMOVED' });
    cy.contains('Status: Removed').should('be.visible');
    cy.contains('button', 'Save changes').should('not.exist'); cy.contains('button', 'Publish job').should('not.exist');
  });
});

describe('JT-73 applicant jobs', () => {
  it('sends search filters, resets pagination, views full details and links to Apply', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, req => {
      req.reply({ jobs: [job], pagination: { nextPageToken: '20', totalCount: 21 } });
    }).as('search');
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { ...job, status: 'JOB_STATUS_PUBLISHED' } });
    login('ROLE_APPLICANT'); open('Search jobs'); cy.wait('@search');
    cy.contains('button', 'Next page').click(); cy.wait('@search').its('request.query').should('include', { 'pagination.page_token': '20' });
    field('Search title').type(' intern '); choose('Category', 'Technology'); choose('Tags', 'React'); field('Tags').type('{esc}'); field('Location').type('Thimphu{esc}');
    cy.contains('button', exact('Search')).click();
    cy.wait('@search').its('request.query').should('include', { query: 'intern', category_id: categoryId, tag_ids: tagId, location: 'Thimphu', 'pagination.page_token': '' });
    card(job.title).contains('a', 'View job').click(); cy.contains(job.description).should('be.visible');
    cy.contains('a', 'Apply for this job').should('have.attr', 'href', `/applicant/apply?jobId=${jobId}`);
    cy.contains('a', 'Report job').should('have.attr', 'href', `/applicant/report-job/${jobId}`);
  });

  it('saves from search, lists the bookmark, views it, and unsaves it', () => {
    let saved = [];
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [job], pagination: {} });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { ...job, status: 'JOB_STATUS_PUBLISHED' } });
    cy.intercept('POST', `${gateway}/v1/saved-jobs`, req => {
      expect(req.body).to.deep.eq({ jobId }); saved = [{ jobId, applicantId: userId, savedAt: deadline }]; req.reply({ savedJob: saved[0] });
    }).as('save');
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, req => req.reply({ savedJobs: saved, pagination: { totalCount: saved.length } }));
    cy.intercept('DELETE', `${gateway}/v1/saved-jobs/${jobId}`, req => { saved = []; req.reply({ statusCode: 200, body: {} }); }).as('unsave');
    login('ROLE_APPLICANT'); open('Search jobs'); card(job.title).contains('button', exact('Save job')).click(); cy.wait('@save');
    card(job.title).should('contain', 'Job saved.');
    open('Saved jobs'); card(jobId).contains('a', 'View job').click(); cy.contains(job.description).should('be.visible');
    open('Saved jobs'); card(jobId).contains('button', 'Unsave job').click(); cy.wait('@unsave');
    cy.contains('No saved jobs on this page.').should('be.visible');
  });

  it('unsaves a newly saved job directly from its details', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { ...job, status: 'JOB_STATUS_PUBLISHED' } });
    cy.intercept('POST', `${gateway}/v1/saved-jobs`, { savedJob: { jobId } });
    cy.intercept('DELETE', `${gateway}/v1/saved-jobs/${jobId}`, { statusCode: 200, body: {} }).as('unsave');
    login('ROLE_APPLICANT'); visit(`/applicant/jobs/${jobId}`);
    cy.contains('button', exact('Save job')).click(); cy.contains('button', 'Unsave job').click(); cy.wait('@unsave');
    cy.contains('button', exact('Save job')).should('be.enabled');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already saved'], [400, 9, 'closed'], [503, 14, 'request failed']]) {
    it(`shows a safe save failure ${code}`, () => {
      cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [job], pagination: {} });
      cy.intercept('POST', `${gateway}/v1/saved-jobs`, { statusCode, body: { code, message: 'private-input' } });
      login('ROLE_APPLICANT'); open('Search jobs'); cy.contains('button', exact('Save job')).click();
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
    });
  }

  for (const value of [{ ...job, status: 'JOB_STATUS_CLOSED' }, { ...job, status: 'JOB_STATUS_PUBLISHED', deadline: '2020-01-01T00:00:00Z' }]) {
    it(`blocks save and apply on a ${value.status} job with deadline ${value.deadline}`, () => {
      cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: value });
      login('ROLE_APPLICANT'); visit(`/applicant/jobs/${jobId}`);
      cy.contains('past its deadline').should('be.visible'); cy.contains('button', exact('Save job')).should('not.exist');
      cy.contains('a', 'Apply for this job').should('not.exist');
    });
  }

  it('handles an unavailable job and invalid address safely', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    login('ROLE_APPLICANT'); visit(`/applicant/jobs/${jobId}`);
    cy.get('[role=alert]').should('contain', 'not found').and('not.contain', 'private-input');
    visit('/applicant/jobs/bad-id'); cy.contains('job ID in the address is not valid').should('be.visible');
  });

  it('clears search filters and shows an empty result', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} }).as('search');
    login('ROLE_APPLICANT'); open('Search jobs'); cy.wait('@search');
    field('Search title').type('Nothing'); cy.contains('button', exact('Search')).click(); cy.wait('@search');
    cy.contains('No open jobs match these filters.').should('be.visible');
    cy.contains('button', 'Clear filters').click(); cy.wait('@search').its('request.query').should('include', { query: '', category_id: '', location: '', 'pagination.page_token': '' });
    field('Search title').should('have.value', '');
  });
});

describe('JT-73 notifications and routing', () => {
  for (const role of ['ROLE_APPLICANT', 'ROLE_EMPLOYER', 'ROLE_ADMIN']) {
    it(`lists and marks notifications read for ${role}`, () => {
      let read = false;
      cy.intercept('GET', `${gateway}/v1/notifications?*`, req => {
        req.reply({ notifications: req.query.unread_only === 'true' && read ? [] : [{ ...notice, read }], pagination: { totalCount: req.query.unread_only === 'true' && read ? 0 : 1 } });
      }).as('notifications');
      cy.intercept('POST', `${gateway}/v1/notifications/${notificationId}/read`, req => {
        expect(req.body).to.deep.eq({}); read = true; req.reply({ notification: { ...notice, read } });
      }).as('read');
      login(role); open('Notifications'); cy.wait('@notifications');
      card(notice.title).should('contain.text', notice.body).find('script').should('not.exist');
      cy.contains('label', 'Unread only').click(); cy.wait('@notifications').its('request.query.unread_only').should('eq', 'true');
      card(notice.title).contains('button', 'Mark as read').click(); cy.wait('@read');
      cy.contains('No notifications on this page.').should('be.visible'); cy.contains('Unread notifications: 0').should('be.visible');
      cy.contains('label', 'Unread only').click(); card(notice.title).should('contain', 'Read');
      cy.contains('button', 'Mark as read').should('not.exist');
    });
  }

  it('paginates notifications and handles mark-read failures safely', () => {
    cy.intercept('GET', `${gateway}/v1/notifications?*`, req => req.reply({ notifications: [notice], pagination: req.query['pagination.page_token'] ? {} : { nextPageToken: '20' } })).as('list');
    login('ROLE_APPLICANT'); open('Notifications'); cy.wait('@list');
    cy.contains('button', 'Next page').click(); cy.wait('@list').its('request.query').should('include', { 'pagination.page_token': '20' });
    cy.intercept('POST', `${gateway}/v1/notifications/${notificationId}/read`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    cy.contains('button', 'Mark as read').click(); cy.get('[role=alert]').should('contain', 'own notifications').and('not.contain', 'private-input');
    cy.contains('button', 'Mark as read').should('be.enabled');
  });

  it('clears an expired session on a protected notification request', () => {
    cy.intercept('GET', `${gateway}/v1/notifications?*`, { statusCode: 401, body: { code: 16 } });
    login('ROLE_APPLICANT'); open('Notifications'); cy.location('pathname').should('eq', '/login');
  });

  it('restricts job routes and navigation to the correct role', () => {
    login('ROLE_APPLICANT'); cy.contains('nav a', exact('My jobs')).should('not.exist');
    visit('/employer/jobs'); cy.location('pathname').should('eq', '/applicant');
    cy.contains('button', 'Log out').click(); login();
    cy.contains('nav a', exact('Search jobs')).should('not.exist'); cy.contains('nav a', exact('Saved jobs')).should('not.exist');
    visit('/applicant/saved-jobs'); cy.location('pathname').should('eq', '/employer');
    cy.contains('button', 'Log out').click(); login('ROLE_ADMIN');
    visit('/employer/jobs/new'); cy.location('pathname').should('eq', '/admin');
  });

  it('requires login for the new protected pages', () => {
    for (const path of ['/employer/jobs/new', '/applicant/jobs', '/applicant/saved-jobs', '/admin/notifications']) {
      cy.visit(path); cy.location('pathname').should('eq', '/login');
    }
  });
});

describe('JT-73 recovery and boundaries', () => {
  it('does not offer management actions on another employer’s published job', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { ...job, employerId: categoryId, status: 'JOB_STATUS_PUBLISHED' } });
    login(); visit(`/employer/jobs/${jobId}`);
    cy.contains('Only the employer who owns this job').should('be.visible');
    cy.contains('button', 'Save changes').should('not.exist'); cy.contains('button', 'Close job').should('not.exist');
  });

  it('retries missing job options and enforces the tag count before saving', () => {
    cy.intercept('GET', `${gateway}/v1/tags?*`, { statusCode: 503, body: { code: 14, message: 'private-input' } });
    manage();
    cy.contains('button', 'Save changes').should('be.disabled');
    cy.get('[role=alert]').should('not.contain', 'private-input');
    const tags = Array.from({ length: 21 }, (_, i) => ({ tagId: `44444444-4444-4444-8444-${String(i).padStart(12, '0')}`, name: `Skill ${i}` }));
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags, pagination: {} });
    cy.contains('button', 'Retry job options').click();
    // The existing job already carries one tag, retained even if absent from the lookup.
    for (let i = 0; i < 20; i++) { choose('Tags', `Skill ${i}`); field('Tags').type('{esc}'); }
    cy.contains('Choose at most 20 tags.').should('be.visible');
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Tags').type('{backspace}'); cy.contains('button', 'Save changes').should('be.enabled');
  });

  it('validates search text limits and recovers from a network failure', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { forceNetworkError: true });
    login('ROLE_APPLICANT'); open('Search jobs');
    cy.contains('Cannot reach JobTrack.').should('be.visible');
    fill('Search title', 'x'.repeat(151)); cy.contains('button', exact('Search')).should('be.disabled');
    fill('Search title', 'x'.repeat(150)); cy.contains('button', exact('Search')).should('be.enabled');
    fill('Location', 'x'.repeat(101)); cy.contains('button', exact('Search')).should('be.disabled');
    fill('Location', 'x'.repeat(100)); cy.contains('button', exact('Search')).should('be.enabled');
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} });
    cy.contains('button', 'Refresh search').click(); cy.contains('No open jobs match these filters.').should('be.visible');
  });

  it('paginates saved jobs and refreshes after a bookmark disappears', () => {
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, req => req.reply({ savedJobs: [{ jobId, applicantId: userId, savedAt: deadline }], pagination: req.query['pagination.page_token'] ? {} : { nextPageToken: '20' } })).as('saved');
    login('ROLE_APPLICANT'); open('Saved jobs'); cy.wait('@saved');
    cy.contains('button', 'Next page').click(); cy.wait('@saved').its('request.query').should('include', { 'pagination.page_token': '20' });
    cy.intercept('DELETE', `${gateway}/v1/saved-jobs/${jobId}`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    cy.contains('button', 'Unsave job').click(); cy.get('[role=alert]').should('contain', 'no longer available').and('not.contain', 'private-input');
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, { savedJobs: [], pagination: {} });
    cy.contains('button', 'Refresh saved jobs').click(); cy.contains('No saved jobs on this page.').should('be.visible');
  });
});
