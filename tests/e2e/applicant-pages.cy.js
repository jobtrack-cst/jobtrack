// JT-112 applicant pages (redesign doc Section 5), gateway-stubbed. They keep the
// rules the JT-73, JT-78, JT-83, JT-88 and JT-108 specs checked on the pages they replace.
import { settleHome } from './home';
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
const employerId = '66666666-6666-4666-8666-666666666666';
const jobId = '22222222-2222-4222-8222-222222222222';
const otherJobId = '33333333-3333-4333-8333-333333333333';
const categoryId = '10000000-0000-4000-8000-000000000001';
const tagId = '20000000-0000-4000-8000-000000000001';
const resumeId = '30000000-0000-4000-8000-000000000001';
const applicationId = '44444444-4444-4444-8444-444444444444';
const offerId = '55555555-5555-4555-8555-555555555555';
const uuid = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
const day = 86_400_000;
const inDays = days => new Date(Date.now() + days * day).toISOString();
const job = { jobId, employerId, title: 'Frontend intern', description: 'Build accessible pages', categoryId, tagIds: [tagId], location: 'Thimphu', deadline: inDays(20), status: 'JOB_STATUS_PUBLISHED' };
const application = { applicationId, jobId, resumeId, applicantId: userId, employerId, withdrawn: false, timestamps: { createdAt: '2026-11-14T04:00:00Z' } };
const offer = { offerId, applicationId, terms: 'Synthetic offer terms', status: 'OFFER_STATUS_PENDING' };
const resume = (id, fileName, validationStatus) => ({ resumeId: id, fileName, validationStatus, sizeBytes: '9', timestamps: { createdAt: '2026-11-15T10:00:00Z' } });
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
const field = label => cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea, select').first();
// Long text is set with the native value setter and one input event (see JT-83).
function fill(label, text) {
  field(label).then($el => {
    const el = $el[0];
    const view = el.ownerDocument.defaultView;
    const proto = el.tagName === 'TEXTAREA' ? view.HTMLTextAreaElement.prototype : view.HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, text);
    el.dispatchEvent(new view.Event('input', { bubbles: true }));
  });
}
const noUuidOnPage = () => cy.get('main').invoke('text').should('not.match', uuid);
const jobCard = title => cy.contains('ul[aria-label="Jobs"] > li', title);
const dialog = () => cy.get('[role=dialog]');
const open = label => cy.contains('nav a', exact(label)).click();
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}

// Registered before each test body, so every stub a test adds takes precedence;
// nothing reaches a real gateway.
beforeEach(() => {
  cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt112-token');
  });
  cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: { code: 5 } });
  cy.intercept('GET', `${gateway}/v1/companies/${employerId}`, { profile: { employerId, companyName: 'Druk Tech' } });
  cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [{ categoryId, name: 'Technology' }], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId, name: 'React' }], pagination: {} });
  cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: 'l1', name: 'Thimphu' }], pagination: {} });
});

function login(role = 'ROLE_APPLICANT') {
  const user = { userId, fullName: 'Pema Choden', email: 'pema@example.test', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt112-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user }).as('login');
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  // Wait for the login round trip first; the pathname check then covers only the navigation.
  cy.wait('@login');
  cy.location('pathname').should('eq', `/${role.slice(5).toLowerCase()}`);
  settleHome();
}

// Skills: a multiple choice in the search bar; typing filters the options.
function chooseSkill(name) {
  field('Skills').should('be.enabled').click().type(name);
  cy.contains('[role=option]', exact(name)).click();
  field('Skills').type('{esc}');
}
const skillChips = () => cy.get('ul[aria-label="Selected skills"]');

describe('JT-112 find jobs', () => {
  it('sends the search bar filters including skills, resets paging and shows job cards without IDs', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [job], pagination: { nextPageToken: '20', totalCount: 21 } }).as('search');
    login(); open('Find jobs'); cy.wait('@search');
    cy.contains('h2', '21 open roles').should('be.visible');
    jobCard(job.title).should('contain', 'Druk Tech, Thimphu').and('contain', 'React').and('contain', 'Closes');
    cy.contains('button', exact('Next')).click(); cy.wait('@search').its('request.query').should('include', { 'pagination.page_token': '20' });
    field('Job title').type(' intern ');
    cy.contains('label', 'Category').parent().find('select').select('Technology');
    field('Location').type('Thimphu');
    chooseSkill('React');
    cy.get('form[role=search]').contains('button', exact('Search')).click();
    cy.wait('@search').its('request.query').should('include', { query: 'intern', category_id: categoryId, location: 'Thimphu', tag_ids: tagId, 'pagination.page_token': '' });
    cy.contains('h2', '21 open roles in Technology').should('be.visible');
    noUuidOnPage();
  });

  it('limits the title to 150 and the location to 100 characters and recovers from a network failure', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { forceNetworkError: true });
    login(); open('Find jobs');
    cy.contains('Cannot reach JobTrack.').should('be.visible');
    const search = () => cy.get('form[role=search]').contains('button', exact('Search'));
    fill('Job title', 'x'.repeat(151)); search().should('be.disabled'); cy.contains('at most 150 characters').should('be.visible');
    fill('Job title', 'x'.repeat(150)); search().should('be.enabled');
    fill('Location', 'x'.repeat(101)); search().should('be.disabled');
    fill('Location', 'x'.repeat(100)); search().should('be.enabled');
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} });
    cy.contains('button', 'Try again').click(); cy.contains('No open roles match').should('be.visible');
  });

  it('chooses skills from every reference page with checkboxes, shows them as chips and sends none for Any', () => {
    const goId = '20000000-0000-4000-8000-000000000002';
    cy.intercept('GET', `${gateway}/v1/tags?*`, req => req.reply(req.query['pagination.page_token'] === '20'
      ? { tags: [{ tagId: goId, name: 'Go' }], pagination: {} }
      : { tags: [{ tagId, name: 'React' }], pagination: { nextPageToken: '20' } })).as('tags');
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} }).as('search');
    login(); open('Find jobs'); cy.wait('@search');
    field('Skills').should('have.attr', 'placeholder', 'Any');
    skillChips().should('not.exist');
    // Both reference pages are offered; typing filters them.
    field('Skills').click();
    cy.get('[role=option]').should('have.length', 2);
    field('Skills').type('Go');
    cy.get('[role=option]').should('have.length', 1).and('contain', 'Go').click();
    cy.contains('[role=option]', exact('Go')).find('input[type=checkbox]').should('be.checked');
    field('Skills').type('{esc}');
    chooseSkill('React');
    field('Skills').should('have.attr', 'placeholder', '2 selected');
    skillChips().find('li').should('have.length', 2).and('contain', 'Go').and('contain', 'React');
    cy.get('form[role=search]').contains('button', exact('Search')).click();
    cy.wait('@search').its('request.url').should('include', `tag_ids=${goId}`).and('include', `tag_ids=${tagId}`);
    // Removing a chip changes the choice; Search sends it.
    skillChips().contains('li', 'Go').find('.MuiChip-deleteIcon').click();
    field('Skills').should('have.attr', 'placeholder', '1 selected');
    skillChips().contains('li', 'React').find('.MuiChip-deleteIcon').click();
    skillChips().should('not.exist');
    field('Skills').should('have.attr', 'placeholder', 'Any');
    cy.get('form[role=search]').contains('button', exact('Search')).click();
    cy.wait('@search').its('request.query').should('not.have.property', 'tag_ids');
    cy.get('@tags.all').should('have.length', 2);
  });

  it('stacks the four search fields on a phone', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} });
    login(); open('Find jobs');
    cy.viewport(390, 844);
    const left = label => cy.contains('form[role=search] label', exact(label)).then($l => $l[0].getBoundingClientRect().left);
    left('Job title').then(x => {
      for (const label of ['Category', 'Location', 'Skills']) left(label).should('eq', x);
    });
    cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
  });

  it('clears the filters and shows an empty result', () => {
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [], pagination: {} }).as('search');
    login(); open('Find jobs'); cy.wait('@search');
    field('Job title').type('Nothing'); cy.get('form[role=search]').contains('button', exact('Search')).click(); cy.wait('@search');
    cy.contains('No open roles match').should('be.visible');
    cy.contains('button', 'Clear filters').first().click();
    cy.wait('@search').its('request.query').should('include', { query: '', category_id: '', location: '', 'pagination.page_token': '' });
    field('Job title').should('have.value', '');
  });

  it('saves from a card, lists the bookmark on Saved and unsaves it from the job page', () => {
    let saved = [];
    cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [job], pagination: {} });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job });
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, req => req.reply({ savedJobs: saved, pagination: { totalCount: saved.length } }));
    cy.intercept('POST', `${gateway}/v1/saved-jobs`, req => {
      expect(req.body).to.deep.eq({ jobId }); saved = [{ jobId, applicantId: userId, savedAt: inDays(0) }]; req.reply({ savedJob: saved[0] });
    }).as('save');
    cy.intercept('DELETE', `${gateway}/v1/saved-jobs/${jobId}`, req => { saved = []; req.reply({ statusCode: 200, body: {} }); }).as('unsave');
    login(); open('Find jobs');
    jobCard(job.title).contains('button', exact('Save')).click(); cy.wait('@save');
    jobCard(job.title).contains('button', exact('Unsave')).should('be.visible');
    open('Saved');
    cy.contains('ul[aria-label="Saved jobs"] > li', job.title).should('contain', 'Druk Tech, Thimphu').contains('a', exact('View')).click();
    cy.location('pathname').should('eq', `/applicant/jobs/${jobId}`);
    cy.contains('button', exact('Unsave')).click(); cy.wait('@unsave');
    cy.contains('button', exact('Save')).should('be.enabled');
    cy.contains('Job removed from Saved.').should('be.visible');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already saved'], [400, 9, 'closed'], [503, 14, 'request failed']]) {
    it(`shows a safe save failure ${code}`, () => {
      cy.intercept('GET', `${gateway}/v1/search/jobs?*`, { jobs: [job], pagination: {} });
      cy.intercept('POST', `${gateway}/v1/saved-jobs`, { statusCode, body: { code, message: 'private-input' } });
      login(); open('Find jobs'); jobCard(job.title).contains('button', exact('Save')).click();
      jobCard(job.title).find('[role=alert]').should('contain', message).and('not.contain', 'private-input');
    });
  }
});

describe('JT-112 job details', () => {
  const resumes = [resume(resumeId, 'validated.pdf', 'RESUME_VALIDATION_STATUS_VALIDATED'), resume('r2', 'pending.pdf', 'RESUME_VALIDATION_STATUS_PENDING'),
    resume('r3', 'rejected.pdf', 'RESUME_VALIDATION_STATUS_REJECTED')];
  function details(value = job, existing = []) {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: value });
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes, pagination: {} }).as('resumes');
    login();
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query.job_id).to.eq(jobId);
      req.reply({ applications: existing, pagination: {} });
    }).as('mine');
    visit(`/applicant/jobs/${jobId}`);
    cy.wait('@mine');
    cy.contains('h1', value.title).should('be.visible');
  }
  const chooseResume = name => dialog().contains('label', name).click();

  it('shows the job with its tags and applies with a validated resume chosen in a dialog', () => {
    details();
    cy.contains(job.description).should('be.visible');
    cy.get('[aria-label=Tags]').should('contain', 'Technology').and('contain', 'React');
    cy.contains('Druk Tech').should('be.visible');
    noUuidOnPage();
    cy.contains('main button', exact('Apply')).click(); cy.wait('@resumes');
    dialog().find('input[type=radio]').should('have.length', 1);
    dialog().should('contain', 'validated.pdf').and('not.contain', 'rejected.pdf').and('contain', 'One resume is still being checked.');
    dialog().contains('button', 'Send application').should('be.disabled');
    chooseResume('validated.pdf');
    cy.intercept('POST', `${gateway}/v1/applications`, req => { expect(req.body).to.deep.eq({ jobId, resumeId }); req.reply({ application }); }).as('submit');
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [application], pagination: {} }).as('mine');
    dialog().contains('button', 'Send application').click(); cy.wait('@submit');
    dialog().should('not.exist');
    cy.contains('Application sent.').should('be.visible');
    cy.wait('@mine');
    cy.contains('a', 'View application').should('have.attr', 'href', `/applicant/applications/${applicationId}`);
    cy.contains('main button', exact('Apply')).should('not.exist');
    cy.contains('Applied').should('be.visible');
  });

  it('shows the final status and View application for a past application instead of Apply', () => {
    details(job, [{ ...application, withdrawn: true }]);
    cy.contains('a', 'View application').should('be.visible');
    cy.contains('.MuiChip-label', exact('Withdrawn')).should('be.visible');
    cy.contains('main button', exact('Apply')).should('not.exist');
  });

  for (const value of [{ ...job, status: 'JOB_STATUS_CLOSED' }, { ...job, deadline: '2020-01-01T00:00:00Z' }]) {
    it(`keeps Save and Apply disabled on a ${value.status} job with deadline ${value.deadline}`, () => {
      details(value);
      cy.contains('past its deadline').should('be.visible');
      cy.contains('main button', exact('Apply')).should('be.disabled');
      cy.contains('main button', exact('Save')).should('be.disabled');
    });
  }

  for (const [statusCode, code, expected] of [[400, 9, 'resume is not validated'], [409, 6, 'already applied'], [403, 7, 'permission'], [404, 5, 'not found']]) {
    it(`shows safe apply error ${code} and permits retry`, () => {
      cy.intercept('POST', `${gateway}/v1/applications`, { statusCode, body: { code, message: 'private-input' } });
      details();
      cy.contains('main button', exact('Apply')).click(); chooseResume('validated.pdf');
      dialog().contains('button', 'Send application').click();
      dialog().find('[role=alert]').should('contain', expected).and('not.contain', 'private-input');
      dialog().contains('button', 'Send application').should('be.enabled');
    });
  }

  it('sends one application while the first is still in flight', () => {
    cy.intercept('POST', `${gateway}/v1/applications`, { delay: 800, body: { application } }).as('submit');
    details();
    cy.contains('main button', exact('Apply')).click(); chooseResume('validated.pdf');
    dialog().contains('button', 'Send application').click();
    dialog().contains('button', 'Please wait').should('be.disabled');
    cy.wait('@submit'); cy.get('@submit.all').should('have.length', 1);
  });

  it('loads validated resumes from a later page', () => {
    details();
    cy.intercept('GET', `${gateway}/v1/resumes?*`, req => req.reply(req.query['pagination.page_token'] === '20'
      ? { resumes: [resume(resumeId, 'later.pdf', 'RESUME_VALIDATION_STATUS_VALIDATED')], pagination: {} }
      : { resumes: [], pagination: { nextPageToken: '20' } })).as('pages');
    cy.contains('main button', exact('Apply')).click();
    dialog().contains('label', 'later.pdf').should('be.visible');
    cy.get('@pages.all').should('have.length', 2);
  });

  it('links to the resumes tab when there is no validated resume, and retries a failed list', () => {
    details();
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { statusCode: 503, body: {} });
    cy.contains('main button', exact('Apply')).click();
    dialog().should('contain', 'The request failed');
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [], pagination: {} });
    dialog().contains('button', 'Try again').click();
    dialog().contains('a', 'Upload a resume').should('have.attr', 'href', '/applicant/profile?tab=resumes');
    dialog().contains('button', 'Send application').should('be.disabled');
  });

  it('reports the job from a dialog with a trimmed reason of 1 to 1000 characters', () => {
    details();
    cy.contains('button', 'Report this job').click();
    dialog().contains('button', 'Send report').should('be.disabled');
    fill('Reason', 'x'.repeat(1001));
    dialog().find('[role=alert]').should('contain', 'at most 1000 characters');
    dialog().contains('button', 'Send report').should('be.disabled');
    fill('Reason', 'x'.repeat(1000));
    dialog().contains('button', 'Send report').should('be.enabled');
    fill('Reason', '  Asks applicants for a fee  ');
    cy.intercept('POST', `${gateway}/v1/jobs/${jobId}/reports`, req => {
      expect(req.body).to.deep.eq({ reason: 'Asks applicants for a fee' });
      req.reply({ report: { reportId: offerId, jobId, status: 'REPORT_STATUS_OPEN' } });
    }).as('report');
    dialog().contains('button', 'Send report').click(); cy.wait('@report');
    dialog().should('not.exist');
    cy.contains('Report sent').should('be.visible');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already have an open report'], [400, 9, 'already been removed'], [400, 3, 'not accepted']]) {
    it(`explains a ${statusCode} answer with code ${code} to Send report`, () => {
      cy.intercept('POST', `${gateway}/v1/jobs/${jobId}/reports`, { statusCode, body: { code, message: 'private-input' } });
      details();
      cy.contains('button', 'Report this job').click();
      field('Reason').type('Fake job');
      dialog().contains('button', 'Send report').click();
      dialog().find('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      dialog().contains('button', 'Send report').should('be.enabled');
    });
  }

  it('handles an unavailable job and an address that is not a job ID', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    login(); visit(`/applicant/jobs/${jobId}`);
    cy.get('[role=alert]').should('contain', 'not found').and('not.contain', 'private-input');
    let called = false;
    cy.intercept('GET', `${gateway}/v1/jobs/bad-id`, () => { called = true; });
    visit('/applicant/jobs/bad-id'); cy.contains('This job does not exist').should('be.visible');
    cy.then(() => expect(called).to.eq(false));
  });
});

describe('JT-112 saved jobs', () => {
  it('shows small cards with title, company, location and deadline, pages through them, and shows no IDs', () => {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: { ...job, deadline: inDays(2) } });
    cy.intercept('GET', `${gateway}/v1/jobs/${otherJobId}`, { statusCode: 404, body: { code: 5 } });
    login();
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, req => req.reply(req.query['pagination.page_token'] === '20'
      ? { savedJobs: [{ jobId: otherJobId, applicantId: userId }], pagination: { totalCount: 21 } }
      : { savedJobs: [{ jobId, applicantId: userId }], pagination: { nextPageToken: '20', totalCount: 21 } })).as('saved');
    open('Saved'); cy.wait('@saved');
    cy.contains('ul[aria-label="Saved jobs"] > li', job.title).should('contain', 'Druk Tech, Thimphu').and('contain', 'Closes in 2 days')
      .contains('a', exact('View')).should('have.attr', 'href', `/applicant/jobs/${jobId}`);
    noUuidOnPage();
    cy.contains('button', exact('Next')).click(); cy.wait('@saved').its('request.query').should('include', { 'pagination.page_token': '20' });
    cy.contains('ul[aria-label="Saved jobs"] > li', 'Job no longer available').should('be.visible');
    cy.contains('Showing 21 to 21 of 21').should('be.visible');
  });

  it('says when nothing is saved', () => {
    login();
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, { savedJobs: [], pagination: {} });
    open('Saved'); cy.contains('No saved jobs').should('be.visible');
    cy.contains('.workspace-content a', 'Find jobs').should('have.attr', 'href', '/applicant/jobs');
  });
});

describe('JT-112 my applications', () => {
  const app = (id, forJob, createdAt, extra = {}) => ({ ...application, applicationId: id, jobId: forJob, timestamps: { createdAt }, ...extra });
  const active = app('55555555-5555-4555-8555-555555555551', jobId, '2026-11-14T04:00:00Z');
  const older = app('55555555-5555-4555-8555-555555555552', jobId, '2026-11-01T04:00:00Z', { withdrawn: true });
  const rejected = app('55555555-5555-4555-8555-555555555553', otherJobId, '2026-11-10T04:00:00Z');
  function stubs() {
    cy.intercept('GET', `${gateway}/v1/applications/${active.applicationId}/status`, { statusRecord: { status: 'APPLICATION_STATUS_SHORTLISTED' } });
    cy.intercept('GET', `${gateway}/v1/applications/${rejected.applicationId}/status`, { statusRecord: { status: 'APPLICATION_STATUS_REJECTED' } });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job });
    cy.intercept('GET', `${gateway}/v1/jobs/${otherJobId}`, { job: { ...job, jobId: otherJobId, title: 'Data intern', location: 'Paro' } });
  }

  it('shows one card per job in Active and Past tabs, from every page, without IDs', () => {
    stubs(); login();
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      expect(req.query).not.to.have.property('job_id');
      req.reply(req.query['pagination.page_token'] === '20'
        ? { applications: [rejected], pagination: { totalCount: 3 } }
        : { applications: [older, active], pagination: { nextPageToken: '20', totalCount: 3 } });
    }).as('applications');
    open('Applications');
    cy.contains('[role=tab]', 'Active (1)').should('have.attr', 'aria-selected', 'true');
    cy.get('ul[aria-label="Active applications"] > li').should('have.length', 1).first()
      .should('contain', 'Frontend intern').and('contain', 'Druk Tech, Thimphu').and('contain', 'Shortlisted')
      .contains('a', exact('View')).should('have.attr', 'href', `/applicant/applications/${active.applicationId}`);
    noUuidOnPage();
    cy.contains('[role=tab]', 'Past (1)').click();
    cy.location('search').should('eq', '?tab=past');
    cy.get('ul[aria-label="Past applications"] > li').should('have.length', 1).and('contain', 'Data intern').and('contain', 'Druk Tech, Paro').and('contain', 'Rejected');
    cy.get('@applications.all').should('have.length', 2);
  });

  it('pages through more than 20 cards', () => {
    login();
    const many = Array.from({ length: 21 }, (_, i) => app(`55555555-5555-4555-8555-${String(i).padStart(12, '0')}`, `22222222-2222-4222-8222-${String(i).padStart(12, '0')}`, inDays(-i)));
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: many, pagination: { totalCount: 21 } }).as('applications');
    // Each card reads its job and status. Stub them all, so the list never waits on the
    // stopped gateway refusing 42 requests.
    cy.intercept('GET', `${gateway}/v1/jobs/*`, { job });
    cy.intercept('GET', `${gateway}/v1/applications/*/status`, { statusRecord: { status: 'APPLICATION_STATUS_SUBMITTED' } });
    open('Applications');
    cy.wait('@applications');
    cy.get('[role=status][aria-label="Loading applications"]').should('not.exist');
    cy.get('ul[aria-label="Active applications"] > li').should('have.length', 20);
    cy.contains('Showing 1 to 20 of 21').should('be.visible');
    cy.contains('button', exact('Next')).click();
    cy.get('ul[aria-label="Active applications"] > li').should('have.length', 1);
    cy.contains('button', exact('Previous')).click();
    cy.get('ul[aria-label="Active applications"] > li').should('have.length', 20);
  });

  it('says when there are no applications', () => {
    login();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [], pagination: {} });
    open('Applications');
    cy.contains('No active applications').should('be.visible');
    cy.contains('[role=tab]', 'Past').click(); cy.contains('No past applications').should('be.visible');
  });

  it('redirects to login when the session expires on a protected request', () => {
    login();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { statusCode: 401, body: { code: 16 } });
    open('Applications');
    cy.location('pathname').should('eq', '/login');
  });
});

describe('JT-115 item 2: an application opens at the top', () => {
  it('opens the application page at scrollY 0 after scrolling Applications, with the back link below the top bar', () => {
    // Eight applications for eight jobs (one card per job).
    const apps = Array.from({ length: 8 }, (_, i) => ({ ...application, applicationId: `44444444-4444-4444-8444-${String(i).padStart(12, '0')}`,
      jobId: `22222222-2222-4222-8222-${String(i).padStart(12, '0')}` }));
    const target = apps[7];
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: apps, pagination: { totalCount: apps.length } }).as('applications');
    cy.intercept('GET', `${gateway}/v1/applications/*/status`, { statusRecord: { status: 'APPLICATION_STATUS_SUBMITTED' } });
    cy.intercept('GET', `${gateway}/v1/jobs/*`, { job });
    cy.intercept('GET', `${gateway}/v1/applications/${target.applicationId}`, { application: target });
    cy.intercept('GET', `${gateway}/v1/applications/${target.applicationId}/offer`, { statusCode: 404, body: { code: 5 } });
    login();
    // Open the list with the desktop links, then narrow the window (under 900px the links
    // are replaced by the Menu button).
    open('Applications');
    cy.wait('@applications');
    cy.get('ul[aria-label="Active applications"] > li').should('have.length', apps.length);
    cy.viewport(390, 700);
    cy.get('button[aria-label="Open navigation"]').should('be.visible');
    cy.window().then(win => win.scrollTo(0, win.document.documentElement.scrollHeight));
    cy.window().its('scrollY').should('be.greaterThan', 0);
    cy.get('ul[aria-label="Active applications"] > li').last().contains('a', exact('View')).click();
    cy.location('pathname').should('eq', `/applicant/applications/${target.applicationId}`);
    cy.contains('main h1', job.title).should('be.visible');
    cy.window().its('scrollY').should('eq', 0);
    cy.get('header').then($bar => {
      cy.contains('main a', 'Applications').then($back => {
        expect($back[0].getBoundingClientRect().top).to.be.at.least($bar[0].getBoundingClientRect().bottom);
      });
    });
  });
});

describe('JT-112 application page', () => {
  const base = `${gateway}/v1/applications/${applicationId}`;
  const history = [
    { previousStatus: 'APPLICATION_STATUS_UNSPECIFIED', newStatus: 'APPLICATION_STATUS_SUBMITTED', occurredAt: '2026-11-14T10:00:00Z' },
    { previousStatus: 'APPLICATION_STATUS_SUBMITTED', newStatus: 'APPLICATION_STATUS_UNDER_REVIEW', occurredAt: '2026-11-15T09:00:00Z' },
    { previousStatus: 'APPLICATION_STATUS_UNDER_REVIEW', newStatus: 'APPLICATION_STATUS_SHORTLISTED', occurredAt: '2026-11-15T10:00:00Z' },
  ];
  const message = (n, senderId = employerId) => ({ messageId: `m${n}`, senderId, content: `Message ${n}`, timestamps: { createdAt: '2026-11-16T04:00:00Z' } });
  // state.status, state.offer and state.messages can be changed during a test.
  function openPage(state = {}) {
    const s = { status: 'APPLICATION_STATUS_SHORTLISTED', offer: null, messages: [message(1), message(2, userId)], ...state };
    cy.intercept('GET', base, { application });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job });
    cy.intercept('GET', `${base}/status`, req => req.reply({ statusRecord: { applicationId, status: s.status } })).as('status');
    cy.intercept('GET', `${base}/status-history?*`, req => req.reply(req.query['pagination.page_token'] === '20'
      ? { entries: history.slice(2), pagination: {} } : { entries: history.slice(0, 2), pagination: { nextPageToken: '20' } })).as('history');
    cy.intercept('GET', `${base}/interviews?*`, { interviews: [{ interviewId: 'i1', applicationId, scheduledAt: '2026-11-20T04:00:00Z', details: 'Room 2\nBring your ID' }], pagination: {} });
    cy.intercept('GET', `${base}/offer`, req => req.reply(s.offer ? { offer: s.offer } : { statusCode: 404, body: { code: 5 } })).as('offer');
    cy.intercept('GET', `${base}/messages?*`, req => req.reply({ messages: s.messages, pagination: { totalCount: s.messages.length } })).as('messages');
    login();
    visit(`/applicant/applications/${applicationId}`);
    cy.contains('h1', job.title).should('be.visible');
    return s;
  }
  const section = name => cy.get(`section[aria-label="${name}"]`);

  it('shows the job, status rail, full history, interviews, no offer and the conversation, without IDs', () => {
    openPage();
    cy.contains('a', 'Applications').should('have.attr', 'href', '/applicant/applications');
    section('Job').should('contain', 'Druk Tech').and('contain', 'Thimphu').and('contain', job.description);
    section('Status').find('ol[aria-label^="Application progress: Step 3 of 5, Shortlisted"] [aria-current=step]').should('exist');
    // Both history pages are read, oldest first.
    section('Status').find('ul[aria-label="Status history"] > li').should('have.length', 3).first().should('contain', 'Submitted from None');
    section('Interviews').should('contain', 'Room 2');
    section('Offer').should('contain', 'No offer yet');
    section('Conversation').find('[role=log]').should('contain', 'Druk Tech').and('contain', 'Message 1').and('contain', 'You');
    noUuidOnPage();
  });

  it('withdraws only after confirmation, then shows the application as view only', () => {
    openPage();
    cy.intercept('POST', `${base}/withdraw`, req => { expect(req.body).to.deep.eq({}); req.reply({ application: { ...application, withdrawn: true } }); }).as('withdraw');
    section('Status').contains('button', 'Withdraw').click();
    dialog().should('contain', 'cannot apply for this job again').contains('button', 'Cancel').click();
    cy.get('@withdraw.all').should('have.length', 0);
    section('Status').contains('button', 'Withdraw').click();
    dialog().contains('button', exact('Withdraw')).click(); cy.wait('@withdraw');
    cy.contains('Application withdrawn.').should('be.visible');
    section('Status').contains('.MuiChip-label', exact('Withdrawn')).should('be.visible');
    section('Status').contains('button', 'Withdraw').should('not.exist');
    section('Conversation').should('contain', 'view only').find('textarea').should('not.exist');
  });

  it('offers no Withdraw in a final state', () => {
    openPage({ status: 'APPLICATION_STATUS_REJECTED' });
    section('Status').contains('.MuiChip-label', exact('Rejected')).should('be.visible');
    section('Status').contains('button', 'Withdraw').should('not.exist');
  });

  for (const decision of ['accept', 'decline']) {
    it(`confirms ${decision} of the offer loaded by application; a final offer cannot be answered again`, () => {
      const s = openPage({ status: 'APPLICATION_STATUS_OFFERED', offer });
      cy.wait('@offer');
      section('Offer').should('contain', offer.terms).and('contain', 'Awaiting reply');
      const label = decision === 'accept' ? 'Accept offer' : 'Decline offer';
      cy.intercept('POST', `${gateway}/v1/offers/${offerId}/${decision}`, req => {
        expect(req.body).to.deep.eq({});
        s.offer = { ...offer, status: decision === 'accept' ? 'OFFER_STATUS_ACCEPTED' : 'OFFER_STATUS_DECLINED' };
        s.status = decision === 'accept' ? 'APPLICATION_STATUS_ACCEPTED' : 'APPLICATION_STATUS_DECLINED';
        req.reply({ offer: s.offer });
      }).as('decision');
      section('Offer').contains('button', label).click();
      dialog().should('contain', 'This decision is final').contains('button', label).click(); cy.wait('@decision');
      section('Offer').should('contain', decision === 'accept' ? 'Accepted' : 'Declined');
      section('Offer').contains('button', 'Accept offer').should('not.exist');
      section('Offer').contains('button', 'Decline offer').should('not.exist');
      section('Status').contains('button', 'Withdraw').should('not.exist');
    });
  }

  it('keeps a pending offer available after a failed decision for retry', () => {
    openPage({ status: 'APPLICATION_STATUS_OFFERED', offer });
    section('Offer').contains('button', 'Accept offer').click();
    cy.intercept('POST', `${gateway}/v1/offers/${offerId}/accept`, { statusCode: 503, body: { message: 'private-input' } }).as('failed');
    dialog().contains('button', 'Accept offer').click(); cy.wait('@failed');
    dialog().find('[role=alert]').should('contain', 'The request failed').and('not.contain', 'private-input');
    cy.intercept('POST', `${gateway}/v1/offers/${offerId}/accept`, { offer: { ...offer, status: 'OFFER_STATUS_ACCEPTED' } }).as('accepted');
    dialog().contains('button', 'Accept offer').should('be.enabled').click(); cy.wait('@accepted');
    dialog().should('not.exist');
    cy.contains('Offer accepted.').should('be.visible');
  });

  it('shows an offer failure apart from no offer and allows retry', () => {
    openPage();
    cy.intercept('GET', `${base}/offer`, { statusCode: 503, body: { code: 14, message: 'private-details' } });
    cy.reload();
    section('Offer').find('[role=alert]').should('contain', 'The request failed').and('not.contain', 'private-details');
    section('Offer').should('not.contain', 'No offer yet');
    cy.intercept('GET', `${base}/offer`, { offer });
    section('Offer').contains('button', 'Try again').click();
    section('Offer').should('contain', offer.terms);
  });

  it('shows the newest messages from totalCount and loads earlier ones on request', () => {
    const all = Array.from({ length: 25 }, (_, i) => message(i + 1));
    openPage({ messages: [] });
    cy.intercept('GET', `${base}/messages?*`, req => {
      const offset = Number(req.query['pagination.page_token'] || 0);
      req.reply({ messages: all.slice(offset, offset + 20), pagination: { totalCount: 25, nextPageToken: offset + 20 < 25 ? String(offset + 20) : '' } });
    }).as('pages');
    cy.reload();
    section('Conversation').find('[role=log]').should('contain', 'Message 25').and('contain', 'Message 6').and('not.contain', 'Message 5');
    cy.get('@pages.all').then(calls => expect(calls.map(c => c.request.query['pagination.page_token'])).to.deep.eq(['', '5']));
    section('Conversation').contains('button', 'Show earlier messages').click();
    section('Conversation').find('[role=log] > div').should('have.length', 25).first().should('contain', 'Message 1');
    section('Conversation').contains('button', 'Show earlier messages').should('not.exist');
  });

  it('sends a trimmed message, keeps the draft on failure and limits it to 2000 characters', () => {
    openPage({ messages: [] });
    section('Conversation').should('contain', 'No messages yet');
    section('Conversation').contains('button', 'Send message').should('be.disabled');
    fill('Message', 'x'.repeat(2001));
    section('Conversation').contains('button', 'Send message').should('be.disabled');
    fill('Message', 'x'.repeat(2000));
    section('Conversation').contains('button', 'Send message').should('be.enabled');
    fill('Message', '  Retry this draft  ');
    cy.intercept('POST', `${base}/messages`, { statusCode: 503, body: {} });
    section('Conversation').contains('button', 'Send message').click();
    section('Conversation').find('[role=alert]').should('contain', 'The request failed');
    field('Message').should('have.value', '  Retry this draft  ');
    cy.intercept('POST', `${base}/messages`, req => {
      expect(req.body).to.deep.eq({ content: 'Retry this draft' });
      req.reply({ message: { messageId: 'sent', senderId: userId, content: 'Retry this draft' } });
    }).as('send');
    section('Conversation').contains('button', 'Send message').click(); cy.wait('@send');
    field('Message').should('have.value', '');
    section('Conversation').find('[role=log]').should('contain', 'Retry this draft');
  });

  it('blocks sending when the thread cannot be read', () => {
    openPage();
    cy.intercept('GET', `${base}/messages?*`, { statusCode: 403, body: { code: 7 } });
    cy.reload();
    section('Conversation').should('contain', 'You do not have permission');
    field('Message').type('Cannot send');
    section('Conversation').contains('button', 'Send message').should('be.disabled');
  });

  it('explains a status read that is not allowed', () => {
    openPage();
    cy.intercept('GET', `${base}/status`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    cy.intercept('GET', `${base}/status-history?*`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    cy.reload();
    section('Status').find('[role=alert]').should('have.length', 1).and('contain', 'Only the applicant and the employer').and('not.contain', 'private-input');
  });

  it('rejects an address that is not an application ID without calling the gateway', () => {
    let called = false;
    cy.intercept('GET', `${gateway}/v1/applications/not-a-uuid*`, () => { called = true; });
    login(); visit('/applicant/applications/not-a-uuid');
    cy.contains('This application does not exist').should('be.visible');
    cy.then(() => expect(called).to.eq(false));
  });

  it('sends the old status, interviews and offers addresses to the application page, and messages to the inbox thread', () => {
    login();
    for (const path of [`/applicant/applications/${applicationId}/status`, `/applicant/applications/${applicationId}/interviews`,
      `/applicant/offers?applicationId=${applicationId}`]) {
      visit(path); cy.location('pathname').should('eq', `/applicant/applications/${applicationId}`);
    }
    // JT-113: the old messages address is now the inbox thread itself.
    visit(`/applicant/messages/${applicationId}`); cy.location('pathname').should('eq', `/applicant/messages/${applicationId}`);
    cy.contains('main h1', /^Messages$/).should('be.visible');
    for (const [path, target] of [[`/applicant/report-job/${jobId}`, `/applicant/jobs/${jobId}`], [`/applicant/apply?jobId=${jobId}`, `/applicant/jobs/${jobId}`],
      ['/applicant/apply', '/applicant/jobs'], ['/applicant/saved-jobs', '/applicant/saved'], ['/applicant/offers?applicationId=bad', '/applicant/applications']]) {
      visit(path); cy.location('pathname').should('eq', target);
    }
    visit('/applicant/resumes'); cy.location('pathname').should('eq', '/applicant/profile'); cy.location('search').should('eq', '?tab=resumes');
  });
});

describe('JT-112 resumes under Profile', () => {
  const pdfBase64 = Cypress.Buffer.from('%PDF-1.7\n').toString('base64');
  function openResumes() {
    login();
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { applicantId: userId } });
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [resume(resumeId, 'cv.pdf', 'RESUME_VALIDATION_STATUS_VALIDATED')], pagination: {} });
    open('Profile');
    cy.contains('[role=tab]', exact('Resumes')).click();
    cy.location('search').should('eq', '?tab=resumes');
    cy.contains('cv.pdf').should('be.visible');
    noUuidOnPage();
  }
  const tab = () => ({ opener: {}, closed: false, document: { title: '' }, location: { href: '' }, close: cy.stub().as('closeTab') });

  it('opens a resume with View PDF in a tab opened during the click', () => {
    openResumes();
    const fake = tab();
    cy.window().then(win => cy.stub(win, 'open').returns(fake).as('open'));
    cy.intercept('GET', `${gateway}/v1/resumes/${resumeId}/content`, { fileName: 'cv.pdf', contentType: 'application/pdf', content: pdfBase64 }).as('download');
    cy.contains('button', 'View PDF').click(); cy.wait('@download');
    cy.get('@open').should('have.been.calledOnceWith', '', '_blank');
    cy.wrap(fake).its('location.href').should('match', /^blob:/);
    cy.wrap(fake).its('opener').should('eq', null);
  });

  it('offers Download PDF when the browser blocks the new tab', () => {
    openResumes();
    cy.window().then(win => cy.stub(win, 'open').returns(null));
    cy.intercept('GET', `${gateway}/v1/resumes/${resumeId}/content`, { fileName: 'cv.pdf', contentType: 'application/pdf', content: pdfBase64 });
    cy.contains('button', 'View PDF').click();
    cy.contains('a', 'Download PDF').should('have.attr', 'href').and('match', /^blob:/);
    cy.contains('a', 'Download PDF').should('have.attr', 'download', 'cv.pdf');
  });

  it('closes the tab and shows a fixed message when the PDF cannot be read', () => {
    openResumes();
    const fake = tab();
    cy.window().then(win => cy.stub(win, 'open').returns(fake));
    cy.intercept('GET', `${gateway}/v1/resumes/${resumeId}/content`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    cy.contains('button', 'View PDF').click();
    cy.get('[role=alert]').should('contain', 'no longer available').and('not.contain', 'private-input');
    cy.get('@closeTab').should('have.been.calledOnce');
  });
});

describe('JT-112 applicant routes', () => {
  it('keeps the new applicant pages from employers and admins', () => {
    login('ROLE_EMPLOYER');
    cy.contains('nav a', exact('Saved')).should('not.exist');
    for (const path of ['/applicant/saved', `/applicant/applications/${applicationId}`, `/applicant/jobs/${jobId}`]) {
      visit(path); cy.location('pathname').should('eq', '/employer');
    }
  });

  it('requires login for the new pages', () => {
    for (const path of ['/applicant/saved', `/applicant/applications/${applicationId}`, '/applicant/profile?tab=resumes']) {
      cy.visit(path); cy.location('pathname').should('eq', '/login');
    }
  });
});
