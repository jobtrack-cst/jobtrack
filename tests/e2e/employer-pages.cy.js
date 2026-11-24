import { expectSameTextEdge, field, fill } from './fields';
import { settleHome } from './home';
// JT-111 employer pages (redesign Phase 3). Gateway responses are stubbed, so these
// tests check the UI and the requests it sends, not live services. Run them with the
// Compose gateway stopped (redesign doc Section 9).
const gateway = 'http://localhost:8000';
const employerId = '66666666-6666-4666-8666-666666666666';
const timestamps = { createdAt: '2026-11-16T04:00:00Z', updatedAt: '2026-11-16T05:00:00Z' };

function login() {
  const user = { userId: employerId, email: 'dorji@example.test', fullName: 'Dorji', role: 'ROLE_EMPLOYER' };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt111-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user }).as('login');
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt111-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  // Wait for the login round trip first; the pathname check then covers only the navigation.
  cy.wait('@login');
  cy.location('pathname').should('eq', '/employer');
  settleHome();
}
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
const open = link => cy.contains('nav a', exact(link)).click();
// Client-side navigation keeps the signed-in session (it lives in memory and sessionStorage).
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
const uuidPattern = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;

describe('JT-111 job page', () => {
  const jobId = '22222222-2222-4222-8222-222222222222';
  const knownCategory = '10000000-0000-4000-8000-000000000001';
  // A category and a tag that category-tag no longer lists.
  const missingCategory = '10000000-0000-4000-8000-0000000000ff';
  const knownTag = '20000000-0000-4000-8000-000000000001';
  const missingTag = '20000000-0000-4000-8000-0000000000ff';
  const deadline = new Date(Date.now() + 10 * 86400000).toISOString();
  const job = { jobId, employerId, title: 'Data intern', description: 'Synthetic job', categoryId: missingCategory, tagIds: [knownTag, missingTag],
    location: 'Thimphu', deadline, status: 'JOB_STATUS_DRAFT', timestamps };
  function references() {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [{ categoryId: knownCategory, name: 'Software' }], pagination: { totalCount: 1 } });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId: knownTag, name: 'Go' }], pagination: { totalCount: 1 } });
    cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: '30000000-0000-4000-8000-000000000001', name: 'Thimphu' }], pagination: { totalCount: 1 } });
  }

  it('shows an unlisted category as "Unknown category" and keeps it when saving', () => {
    login();
    references();
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job }).as('job');
    visit(`/employer/jobs/${jobId}`);
    cy.wait('@job');
    // Read-only details: names only, never IDs.
    cy.contains('main h3', exact('Category')).parent().should('contain', 'Unknown category');
    cy.contains('main h3', exact('Tags')).parent().should('contain', 'Go').and('contain', 'Unknown tag');
    cy.get('main').invoke('text').should('not.match', uuidPattern);

    cy.contains('button', 'Edit job').click();
    cy.contains('label', exact('Category')).parent().find('[role=combobox]').should('have.text', 'Unknown category');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Title').type(' updated');
    cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, req => {
      expect(req.body.title).to.eq('Data intern updated');
      // Neither the unknown category nor the unknown tag is cleared.
      expect(req.body.categoryId).to.eq(missingCategory);
      expect(req.body.tagIds).to.deep.eq([knownTag, missingTag]);
      req.reply({ job: { ...job, title: req.body.title } });
    }).as('save');
    cy.contains('button', 'Save changes').click();
    cy.wait('@save');
    cy.contains('Job updated.').should('be.visible');
    cy.contains('main h1', 'Data intern updated').should('be.visible');
    cy.contains('main h3', exact('Category')).parent().should('contain', 'Unknown category');
  });
});

// JT-73 job rules on the JT-111 pages: Jobs tabs, and the job page that opens read-only with
// Edit, Publish and Close (new jobs open straight into the editor).
describe('JT-111 jobs (JT-73 rules)', () => {
  const jobId = '22222222-2222-4222-8222-222222222222';
  const categoryId = '33333333-3333-4333-8333-333333333333';
  const tagId = '44444444-4444-4444-8444-444444444444';
  const deadline = new Date(Date.now() + 7 * 86400000).toISOString();
  const job = { jobId, employerId, title: 'Frontend intern', description: 'Build accessible pages', categoryId, tagIds: [tagId], location: 'Thimphu', deadline, status: 'JOB_STATUS_DRAFT', timestamps };
  function choose(label, option) {
    cy.contains('label', new RegExp(`^${label}`)).parent().find('[role=combobox]').click();
    cy.contains('[role=option]', exact(option)).click();
  }
  beforeEach(() => {
    cy.intercept('GET', `${gateway}/v1/categories?*`, { categories: [{ categoryId, name: 'Technology' }], pagination: {} });
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags: [{ tagId, name: 'React' }], pagination: {} });
    cy.intercept('GET', `${gateway}/v1/locations?*`, { locations: [{ locationId: categoryId, name: 'Thimphu' }], pagination: {} });
    cy.intercept('GET', `${gateway}/v1/companies/**`, { statusCode: 404, body: { code: 5 } });
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [], pagination: { totalCount: 0 } });
  });
  // Opens one job read-only; edit() switches to the form. Another employer's job is not
  // shown, so its page is headed "Job" instead of the title.
  function manage(value = job, heading = value.title) {
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: value });
    login(); visit(`/employer/jobs/${jobId}`);
    cy.contains('main h1', exact(heading)).should('be.visible');
  }
  const edit = () => { cy.contains('main button', exact('Edit job')).click(); cy.contains('button', 'Save changes').should('be.visible'); };
  const statusChip = label => cy.contains('main .MuiChip-label', exact(label)).should('be.visible');
  const dialogReady = () => cy.get('[role=dialog]').should('be.visible').closest('.MuiDialog-container').should('have.css', 'opacity', '1');

  it('lists jobs by tab with pagination and creates, edits, publishes and closes a job', () => {
    let current = { ...job };
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      const mine = current.status === req.query.status ? [current] : [];
      // Drafts has a second, empty page, to check paging.
      if (req.query.status === 'JOB_STATUS_DRAFT') req.reply(req.query['pagination.page_token'] === '20'
        ? { jobs: [], pagination: { totalCount: 1 } } : { jobs: mine, pagination: { nextPageToken: '20', totalCount: 1 } });
      else req.reply({ jobs: mine, pagination: { totalCount: mine.length } });
    }).as('jobs');
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
    login();
    // Home asks for published jobs first; then Jobs asks once per tab.
    cy.wait('@jobs').its('request.query.status').should('eq', 'JOB_STATUS_PUBLISHED');
    open('Jobs');
    cy.wait('@jobs').its('request.query.status').should('eq', 'JOB_STATUS_PUBLISHED');
    cy.contains('No active jobs').should('be.visible');
    cy.contains('[role=tab]', exact('Drafts')).click();
    cy.wait('@jobs').its('request.query').should('include', { status: 'JOB_STATUS_DRAFT', 'pagination.page_token': '' });
    cy.contains('ul[aria-label="Draft jobs"] > li', job.title).should('contain', 'Closes').contains('a', exact('Manage')).should('have.attr', 'href', `/employer/jobs/${jobId}`);
    cy.contains('button', exact('Next')).click();
    cy.wait('@jobs').its('request.query').should('include', { status: 'JOB_STATUS_DRAFT', 'pagination.page_token': '20' });
    cy.contains('main a', 'Post a job').click();
    cy.location('pathname').should('eq', '/employer/jobs/new');
    field('Title').type('  Frontend intern  '); field('Description').type('  Build accessible pages  ');
    choose('Category', 'Technology'); choose('Tags', 'React'); field('Tags').type('{esc}');
    field('Location').type('Thimphu{esc}'); field('Deadline').type('2030-01-20T12:00');
    cy.contains('button', 'Create draft').click(); cy.wait('@create');
    cy.location('pathname').should('eq', `/employer/jobs/${jobId}`);
    // The new draft opens read-only, with names and no IDs.
    cy.contains('Draft created.').should('be.visible');
    statusChip('Draft');
    cy.contains('main h3', exact('Category')).parent().should('contain', 'Technology');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
    edit();
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Title').should('have.value', job.title).clear();
    field('Title').type('Updated intern'); cy.contains('button', 'Save changes').click(); cy.wait('@update');
    cy.contains('Job updated.').should('be.visible');
    cy.contains('main h1', 'Updated intern').should('be.visible');
    cy.contains('main button', exact('Publish job')).click(); dialogReady();
    cy.get('[role=dialog]').contains('button', 'Cancel').click(); cy.get('@publish.all').should('have.length', 0);
    cy.contains('main button', exact('Publish job')).click(); dialogReady();
    cy.get('[role=dialog]').contains('button', 'Confirm publish').click(); cy.wait('@publish');
    statusChip('Published');
    cy.contains('main button', exact('Close job')).click(); dialogReady();
    cy.get('[role=dialog]').contains('button', 'Confirm close').click(); cy.wait('@close');
    statusChip('Closed');
    for (const label of ['Edit job', 'Publish job', 'Close job', 'Save changes']) cy.contains('main button', exact(label)).should('not.exist');
    cy.contains('This job is closed').should('be.visible');
    // Closed jobs are listed under Closed and open read-only with View, never Manage.
    open('Jobs');
    cy.contains('[role=tab]', exact('Closed')).click();
    cy.contains('ul[aria-label="Closed jobs"] > li', 'Updated intern').within(() => {
      cy.contains('.MuiChip-label', exact('Closed')).should('be.visible');
      cy.contains('a', exact('View')).should('be.visible');
      cy.contains('a', exact('Manage')).should('not.exist');
    });
  });

  it('enforces trimmed Unicode text limits and a future local deadline', () => {
    manage(); edit();
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
    manage({ ...job, categoryId: '', status: 'JOB_STATUS_PUBLISHED' });
    cy.contains('main button', exact('Close job')).should('be.visible');
    edit();
    cy.contains('label', /^Category/).parent().find('[role=combobox]').should('have.text', 'No category').and('not.have.attr', 'aria-disabled', 'true');
    choose('Category', 'Technology'); cy.contains('button', 'Save changes').should('be.enabled');
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
      edit();
      field('Description').clear().type('Description-only update');
      cy.contains('button', 'Save changes').click(); cy.wait('@update');
      cy.contains('Job updated.').should('be.visible');
      cy.contains('main h3', exact('Description')).parent().should('contain', 'Description-only update');
      edit();
      field('Description').should('have.value', 'Description-only update');
    });
  }

  it('converts an explicitly changed local deadline to UTC', () => {
    manage(); edit();
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
      const openConfirmation = () => { cy.contains('main button', exact(action === 'publish' ? 'Publish job' : 'Close job')).click(); dialogReady(); };
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
      statusChip(action === 'publish' ? 'Published' : 'Closed');
      cy.get('@transition.all').should('have.length', 3);
    });
  }

  for (const [statusCode, code, message] of [[403, 7, 'owns this job'], [400, 9, 'no longer allowed'], [500, 13, 'request failed']]) {
    it(`safely handles update failure ${code} and preserves entered fields`, () => {
      manage(); edit();
      cy.intercept('PUT', `${gateway}/v1/jobs/${jobId}`, { statusCode, body: { code, message: 'private-input' } });
      field('Title').clear().type('Keep this title'); cy.contains('button', 'Save changes').click();
      cy.get('main [role=alert]').should('contain', message).and('not.contain', 'private-input');
      field('Title').should('have.value', 'Keep this title'); cy.contains('button', 'Save changes').should('be.enabled');
    });
  }

  it('shows removed jobs read-only with full details and no editing or lifecycle actions', () => {
    manage({ ...job, status: 'JOB_STATUS_REMOVED' });
    statusChip('Removed');
    cy.contains('An admin removed this job').should('be.visible');
    cy.contains('main h3', exact('Description')).parent().should('contain', job.description);
    for (const label of ['Edit job', 'Publish job', 'Close job', 'Save changes']) cy.contains('main button', exact(label)).should('not.exist');
  });

  it('does not offer management actions on another employer’s published job', () => {
    manage({ ...job, employerId: categoryId, status: 'JOB_STATUS_PUBLISHED' }, 'Job');
    cy.contains('Only the employer who owns this job').should('be.visible');
    // Nothing of the other employer's job is shown.
    cy.contains('main', job.title).should('not.exist');
    for (const label of ['Edit job', 'Close job', 'Save changes']) cy.contains('main button', exact(label)).should('not.exist');
  });

  it('handles an address that is not a job ID without calling the gateway', () => {
    let called = false;
    cy.intercept('GET', `${gateway}/v1/jobs/not-a-job`, () => { called = true; });
    login(); visit('/employer/jobs/not-a-job');
    cy.contains('This job does not exist').should('be.visible');
    cy.then(() => expect(called).to.eq(false));
  });

  it('retries missing job options and enforces the tag count before saving', () => {
    cy.intercept('GET', `${gateway}/v1/tags?*`, { statusCode: 503, body: { code: 14, message: 'private-input' } });
    manage();
    cy.get('main [role=alert]').should('be.visible').and('not.contain', 'private-input');
    cy.contains('button', 'Save changes').should('not.exist');
    const tags = Array.from({ length: 21 }, (_, i) => ({ tagId: `44444444-4444-4444-8444-${String(i).padStart(12, '0')}`, name: `Skill ${i}` }));
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags, pagination: {} });
    cy.contains('main button', 'Try again').click();
    edit();
    // The existing job already carries one tag, kept (as "Unknown tag") even if absent from the lookup.
    field('Tags').closest('.MuiAutocomplete-root').find('.MuiChip-root').should('have.length', 1).and('contain', 'Unknown tag');
    for (let i = 0; i < 20; i++) { choose('Tags', `Skill ${i}`); field('Tags').type('{esc}'); }
    cy.contains('Choose at most 20 tags.').should('be.visible');
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Tags').type('{backspace}'); cy.contains('button', 'Save changes').should('be.enabled');
  });

  it('keeps twenty wrapped tags clear of usable input and indicator targets at every width', () => {
    const tags = Array.from({ length: 21 }, (_, i) => ({ tagId: `44444444-4444-4444-8444-${String(i).padStart(12, '0')}`, name: `Skill ${i}` }));
    cy.intercept('GET', `${gateway}/v1/tags?*`, { tags, pagination: {} });
    manage({ ...job, tagIds: [tagId, ...tags.slice(0, 19).map(tag => tag.tagId)] });
    edit();
    field('Tags').should('be.enabled');
    for (const width of [320, 390, 768, 1000, 1024, 1280]) {
      cy.viewport(width, 900);
      field('Tags').scrollIntoView().focus().should($input => {
        const input = $input[0].getBoundingClientRect();
        const root = $input[0].closest('.MuiAutocomplete-root');
        for (const button of root.querySelectorAll('.MuiAutocomplete-endAdornment button')) {
          const target = button.getBoundingClientRect();
          expect(input.right, `input ends before ${button.getAttribute('aria-label')} at ${width}px`).to.be.at.most(target.left);
          expect(target.width).to.be.at.least(44);
          expect(target.height).to.be.at.least(44);
        }
      });
      field('Tags').click().should('have.attr', 'aria-expanded', 'true').type('{esc}');
      cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
      cy.env(['jt101JobFormCapture']).then(({ jt101JobFormCapture }) => {
        if (jt101JobFormCapture) {
          cy.get('header').invoke('css', 'position', 'static');
          field('Tags').closest('.MuiAutocomplete-root').screenshot(`job-tags-20-${width}`, { padding: 12, overwrite: true });
          cy.get('header').invoke('css', 'position', '');
        }
      });
    }
    // Keyboard addition/removal keeps the original 20-tag boundary and recovery.
    field('Tags').type('Skill 20{downarrow}{enter}{esc}');
    cy.contains('Choose at most 20 tags.').should('be.visible');
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Tags').type('{backspace}');
    // Back to the job's own 20 tags: the limit message goes, and Save stays disabled
    // because nothing changed (JT-109).
    cy.contains('Choose at most 20 tags.').should('not.exist');
    cy.contains('No changes to save.').should('be.visible');
    cy.contains('button', 'Save changes').should('be.disabled');
    field('Tags').closest('.MuiAutocomplete-root').find('button[aria-label="Clear"]').click();
    field('Tags').closest('.MuiAutocomplete-root').find('.MuiChip-root').should('not.exist');
    field('Tags').type('Skill 0{downarrow}{enter}{esc}');
    field('Tags').closest('.MuiAutocomplete-root').find('.MuiChip-root').should('have.length', 1).and('contain', 'Skill 0');
    cy.contains('button', 'Save changes').should('be.enabled');
  });
});

describe('JT-111 company page', () => {
  const profile = { employerId, companyName: 'Druk Tech', description: 'Synthetic company', website: 'https://druktech.example', location: 'Thimphu',
    verificationStatus: 'VERIFICATION_STATUS_UNVERIFIED', timestamps };

  it('starts multiline and single-line text at the same left edge', () => {
    login();
    cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile }).as('load');
    cy.intercept('GET', `${gateway}/v1/verification/my-requests?*`, { requests: [], pagination: { totalCount: 0 } }).as('requests');
    open('Company');
    cy.wait(['@load', '@requests']);
    cy.contains('button', 'Edit profile').click();
    // Profile form: Company name (single line) and Description (multiline).
    expectSameTextEdge('Company name', 'Description');
    // Verification form: Registration number (single line) and Notes (multiline).
    expectSameTextEdge('Registration number', 'Notes');
  });

  // JT-88 company profile rules on the JT-111 page (read-only with Edit, JT-109 pattern).
  const fields = { companyName: 'Druk Tech', description: 'Synthetic company', website: 'https://druktech.example', location: 'Thimphu' };
  const requestsUrl = `${gateway}/v1/verification/my-requests?*`;
  function openCompany(response, requests = []) {
    login();
    cy.intercept('GET', `${gateway}/v1/employer/profile`, response).as('load');
    cy.intercept('GET', requestsUrl, { requests, pagination: { totalCount: requests.length } }).as('requests');
    open('Company');
    cy.wait('@load');
  }
  const profileView = () => cy.get('main section[aria-label="Company profile"]');
  const verification = () => cy.get('main section[aria-label=Verification]');

  it('creates a profile when the employer has none, sending trimmed fields, then shows it read-only', () => {
    openCompany({ statusCode: 404, body: { code: 5, message: 'private' } });
    cy.contains('You do not have a company profile yet').should('be.visible');
    verification().should('contain', 'Create your company profile first');
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
    profileView().should('contain', 'Druk Tech').and('contain', 'https://druktech.example');
    cy.contains('button', 'Edit profile').should('be.visible');
    cy.contains('label', /^Company name/).should('not.exist');
    verification().find('.MuiChip-root').should('contain', 'Not verified');
    verification().contains('button', 'Send request').should('be.visible');
  });

  it('shows a rejected profile with its reason and saves changes with PUT, then returns to read-only', () => {
    const rejected = { ...profile, verificationStatus: 'VERIFICATION_STATUS_REJECTED', rejectionReason: 'Licence number not found' };
    openCompany({ profile: rejected });
    verification().find('.MuiChip-root').should('contain', 'Rejected');
    verification().should('contain', 'Reason: Licence number not found');
    profileView().should('contain', 'Druk Tech');
    // Read-only until Edit: no profile fields.
    cy.contains('label', /^Company name/).should('not.exist');
    cy.contains('button', 'Edit profile').click();
    field('Company name').should('have.value', 'Druk Tech');
    cy.contains('button', 'Save profile').should('be.disabled');
    field('Location').clear().type('Phuentsholing');
    cy.intercept('PUT', `${gateway}/v1/employer/profile`, req => {
      expect(req.body).to.deep.eq({ ...fields, location: 'Phuentsholing' });
      req.reply({ profile: { ...rejected, location: 'Phuentsholing' } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
    cy.contains('Company profile saved.').should('be.visible');
    profileView().should('contain', 'Phuentsholing');
    cy.contains('button', 'Save profile').should('not.exist');
  });

  it('cancels an edit without saving and restores the saved values (JT-109)', () => {
    openCompany({ profile });
    cy.intercept('PUT', `${gateway}/v1/employer/profile`, () => { throw new Error('Cancel must not save'); });
    cy.contains('button', 'Edit profile').click();
    field('Location').clear().type('Paro');
    cy.contains('button', exact('Cancel')).click();
    profileView().should('contain', 'Thimphu').and('not.contain', 'Paro');
    cy.contains('button', 'Edit profile').click();
    field('Location').should('have.value', 'Thimphu');
  });

  it('shows a verified profile without the request form', () => {
    openCompany({ profile: { ...profile, verificationStatus: 'VERIFICATION_STATUS_VERIFIED' } });
    verification().contains('.MuiChip-root', exact('Verified')).should('be.visible');
    verification().should('contain', 'Your company is verified');
    verification().contains('button', 'Send request').should('not.exist');
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
      cy.contains('button', 'Edit profile').click();
      field('Company name').should('have.value', 'Druk Tech');
      change();
      if (message) cy.get('main [role=alert]').should('contain', message);
      cy.contains('button', 'Save profile').should('be.disabled');
    });
  }

  it('accepts boundary values: 150 character name, 2000 description, 255 website, 100 location', () => {
    openCompany({ profile });
    cy.contains('button', 'Edit profile').click();
    fill('Company name', 'x'.repeat(150));
    fill('Description', 'x'.repeat(2000));
    fill('Website', `https://${'a'.repeat(244)}.bt`);
    fill('Location', 'x'.repeat(100));
    field('Company name').should('have.value', 'x'.repeat(150));
    cy.get('main [role=alert]').should('not.exist');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a fixed message for a rejected save and never the server text', () => {
    cy.intercept('PUT', `${gateway}/v1/employer/profile`, { statusCode: 400, body: { code: 3, message: 'private-input' } });
    openCompany({ profile });
    cy.contains('button', 'Edit profile').click();
    field('Location').clear().type('Paro');
    cy.contains('button', 'Save profile').click();
    cy.get('main [role=alert]').should('contain', 'The profile was not saved').and('not.contain', 'private-input');
    cy.contains('button', 'Save profile').should('be.enabled');
    field('Location').should('have.value', 'Paro');
  });

  it('shows a load failure and retries', () => {
    openCompany({ statusCode: 503, body: {} });
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile });
    cy.contains('button', 'Try again').click();
    profileView().should('contain', 'Druk Tech');
  });

  // JT-88 verification request rules, now in the Verification section of Company.
  const pending = { requestId: '55555555-5555-4555-8555-555555555555', employerId, registrationNumber: 'REG-123', notes: 'Trade licence', status: 'REQUEST_STATUS_PENDING', timestamps };
  const adminId = '99999999-9999-4999-8999-999999999999';
  const requestRow = text => verification().contains('ul[aria-label="Verification requests"] > li', text);

  it('sends a trimmed request and then shows it as pending without the form', () => {
    openCompany({ profile });
    cy.wait('@requests');
    verification().should('contain', 'You have not sent a request yet.');
    verification().contains('button', 'Send request').should('be.disabled');
    field('Registration number').type('  REG-123  ');
    field('Notes').type('Trade licence');
    cy.intercept('POST', `${gateway}/v1/verification/requests`, req => {
      expect(req.body).to.deep.eq({ registrationNumber: 'REG-123', notes: 'Trade licence' });
      req.reply({ request: pending });
    }).as('submit');
    cy.intercept('GET', requestsUrl, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply({ requests: [pending], pagination: { totalCount: 1 } });
    }).as('relist');
    verification().contains('button', 'Send request').click();
    cy.wait(['@submit', '@relist']);
    verification().should('contain', 'Request sent').and('contain', 'An admin is checking your registration');
    verification().contains('.MuiChip-root', exact('Pending')).should('be.visible');
    requestRow('REG-123').should('contain', 'Pending').and('contain', 'Notes: Trade licence').and('not.contain', 'by admin');
    verification().contains('button', 'Send request').should('not.exist');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
  });

  it('offers a new request after a rejection and shows the reason, never the admin ID', () => {
    openCompany({ profile: { ...profile, verificationStatus: 'VERIFICATION_STATUS_REJECTED', rejectionReason: 'Licence number not found' } },
      [{ ...pending, status: 'REQUEST_STATUS_REJECTED', rejectionReason: 'Licence number not found', reviewedBy: adminId }]);
    requestRow('REG-123').should('contain', 'Rejected').and('contain', 'Reason: Licence number not found').and('not.contain', adminId);
    verification().should('contain', 'Send a new request');
    field('Registration number').should('have.value', '');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
  });

  it('shows an approved request without the form', () => {
    openCompany({ profile }, [{ ...pending, status: 'REQUEST_STATUS_APPROVED', reviewedBy: adminId }]);
    cy.wait('@requests');
    verification().should('contain', 'Your request was approved');
    verification().contains('button', 'Send request').should('not.exist');
  });

  it('blocks a 51 character registration number and 1001 character notes, and allows 50 and 1000', () => {
    openCompany({ profile });
    cy.wait('@requests');
    fill('Registration number', 'x'.repeat(51));
    verification().find('[role=alert]').should('contain', 'at most 50 characters');
    verification().contains('button', 'Send request').should('be.disabled');
    fill('Registration number', 'x'.repeat(50));
    fill('Notes', 'x'.repeat(1001));
    verification().find('[role=alert]').should('contain', 'at most 1000 characters');
    verification().contains('button', 'Send request').should('be.disabled');
    fill('Notes', 'x'.repeat(1000));
    verification().find('[role=alert]').should('not.exist');
    verification().contains('button', 'Send request').should('be.enabled');
  });

  for (const [statusCode, code, message] of [
    [409, 6, 'already have a pending request'], [400, 9, 'already verified'], [400, 3, 'was not accepted'], [403, 7, 'Only employers'],
  ]) {
    it(`explains a ${statusCode} answer with code ${code} to Send request`, () => {
      openCompany({ profile });
      cy.wait('@requests');
      cy.intercept('POST', `${gateway}/v1/verification/requests`, { statusCode, body: { code, message: 'private-input' } });
      field('Registration number').type('REG-123');
      verification().contains('button', 'Send request').click();
      verification().find('[role=alert]').should('contain', message).and('not.contain', 'private-input');
      verification().contains('button', 'Send request').should('be.enabled');
    });
  }

  it('sends the old Verification address to Company', () => {
    openCompany({ profile });
    visit('/employer/verification');
    cy.location('pathname').should('eq', '/employer/company');
  });
});

describe('JT-111 applicants page', () => {
  const activeJob = '22222222-2222-4222-8222-222222222222';
  const closedJob = '33333333-3333-4333-8333-333333333333';
  const removedJob = '44444444-4444-4444-8444-444444444444';
  const named = '55555555-5555-4555-8555-555555555555';
  const older = '77777777-7777-4777-8777-777777777777';
  const deadline = new Date(Date.now() + 10 * 86400000).toISOString();
  const jobOf = (jobId, title, status) => ({ jobId, employerId, title, description: 'Synthetic job', location: 'Thimphu', deadline, status, timestamps });
  const jobs = {
    JOB_STATUS_PUBLISHED: [jobOf(activeJob, 'Data intern', 'JOB_STATUS_PUBLISHED')],
    JOB_STATUS_CLOSED: [jobOf(closedJob, 'Web intern', 'JOB_STATUS_CLOSED')],
    JOB_STATUS_REMOVED: [jobOf(removedJob, 'Old intern', 'JOB_STATUS_REMOVED')],
  };
  const app = (applicationId, jobId, applicantName, createdAt = '2026-11-20T04:00:00Z') => ({ applicationId, jobId, applicantId: '11111111-1111-4111-8111-111111111111',
    employerId, resumeId: '88888888-8888-4888-8888-888888888888', applicantName, timestamps: { createdAt } });
  // A 1x1 PNG.
  const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';

  function stubs() {
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, req => {
      const list = jobs[req.query.status];
      expect(list, `status ${req.query.status}`).to.exist;
      req.reply({ jobs: list, pagination: { totalCount: list.length } });
    }).as('jobs');
    for (const job of Object.values(jobs).flat()) cy.intercept('GET', `${gateway}/v1/jobs/${job.jobId}`, { job });
    cy.intercept('GET', `${gateway}/v1/applications/${named}/status`, { statusRecord: { applicationId: named, status: 'APPLICATION_STATUS_SHORTLISTED' } });
    cy.intercept('GET', `${gateway}/v1/applications/${older}/status`, { statusCode: 404, body: { code: 5, message: 'private' } });
    cy.intercept('GET', `${gateway}/v1/applications/${named}/applicant-photo`, { contentType: 'image/png', content: png }).as('photo');
    cy.intercept('GET', `${gateway}/v1/applications/${older}/applicant-photo`, { statusCode: 404, body: { code: 5, message: 'private' } }).as('noPhoto');
  }

  it('shows names, photos or initials, job titles, applied dates and real statuses, with no IDs', () => {
    login();
    stubs();
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query).not.to.have.property('job_id');
      // An application from before JT-114 has no applicant name.
      req.reply({ applications: [app(named, activeJob, 'Pema Choden'), app(older, closedJob, '')], pagination: { totalCount: 2 } });
    }).as('applications');
    cy.window().then(win => cy.spy(win.URL, 'revokeObjectURL').as('revoke'));
    open('Applicants');
    cy.location('pathname').should('eq', '/employer/applicants');
    cy.wait(['@applications', '@photo', '@noPhoto']);
    cy.contains('main h2', exact('2 applicants')).should('be.visible');
    cy.contains('ul[aria-label=Applicants] > li', 'Pema Choden').within(() => {
      cy.contains('Data intern, applied 20 Nov').should('be.visible');
      cy.contains('Shortlisted').should('be.visible');
      cy.get('img').should('have.attr', 'src').and('match', /^blob:/);
      cy.contains('a', 'View').should('have.attr', 'aria-label', 'View application from Pema Choden for Data intern')
        .and('have.attr', 'href', `/employer/applicants/${named}`);
    });
    // No name is invented: "Applicant" with the initial A, and Submitted while no status is recorded yet.
    cy.contains('ul[aria-label=Applicants] > li', 'Web intern, applied 20 Nov').within(() => {
      cy.contains('h3', exact('Applicant')).should('be.visible');
      cy.get('img').should('not.exist');
      cy.contains(exact('A')).should('be.visible');
      cy.contains('Submitted').should('be.visible');
    });
    cy.get('main').invoke('text').should('not.match', uuidPattern);
    // The photo's object URL is revoked when the page unmounts.
    cy.get('ul[aria-label=Applicants] img').invoke('attr', 'src').then(src => {
      open('Jobs');
      cy.get('@revoke').should('have.been.calledWith', src);
    });
  });

  it('filters by job with job_id, labels closed and removed jobs, and pages through results', () => {
    login();
    stubs();
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      // JT-78: the nested pagination query fields.
      expect(req.query['pagination.page_size']).to.eq('20');
      if (req.query.job_id) {
        expect(req.query.job_id).to.eq(closedJob);
        req.reply({ applications: [app(older, closedJob, '')], pagination: { totalCount: 1 } });
      } else req.reply({ applications: [app(named, activeJob, 'Pema Choden')], pagination: { totalCount: 21, nextPageToken: '20' } });
    }).as('applications');
    open('Applicants');
    cy.wait('@applications');
    cy.get('form[role=search] select option').should('have.length', 4).then($options => {
      expect([...$options].map(o => o.textContent)).to.deep.eq(['All jobs', 'Data intern', 'Web intern (Closed)', 'Old intern (Removed)']);
    });
    // Drafts cannot have applicants, so the filter never asks for them.
    cy.get('@jobs.all').then(calls => expect(calls.map(c => c.request.query.status).sort())
      .to.deep.eq(['JOB_STATUS_CLOSED', 'JOB_STATUS_PUBLISHED', 'JOB_STATUS_REMOVED']));
    cy.contains('nav[aria-label=Pages]', 'Showing 1 to 1 of 21').contains('button', exact('Next')).click();
    cy.wait('@applications').its('request.query').should('include', { 'pagination.page_token': '20' });
    cy.get('form[role=search] select').select('Web intern (Closed)');
    cy.get('form[role=search]').contains('button', 'Show applicants').click();
    cy.wait('@applications').its('request.query').should('include', { job_id: closedJob, 'pagination.page_token': '' });
    cy.location('search').should('eq', `?job=${closedJob}`);
    cy.contains('main h2', exact('1 applicant')).should('be.visible');
    cy.contains('For Web intern (Closed)').should('be.visible');
    cy.get('form[role=search] select').select('All jobs');
    cy.get('form[role=search]').contains('button', 'Show applicants').click();
    cy.wait('@applications').its('request.query').should('not.have.property', 'job_id');
    cy.location('search').should('eq', '');
    cy.contains('main h2', exact('21 applicants')).should('be.visible');
    // Exactly one request per step: first load, Next, the chosen job, then All jobs again.
    // A filter change once also sent a stray unfiltered page 1 request before the filtered one.
    cy.get('@applications.all').then(calls => {
      expect(calls.map(c => [c.request.query.job_id ?? '', c.request.query['pagination.page_token']]))
        .to.deep.eq([['', ''], ['', '20'], [closedJob, ''], ['', '']]);
    });
  });

  it('keeps the job from the address and says when it has no applicants', () => {
    login();
    stubs();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [], pagination: { totalCount: 0 } }).as('applications');
    visit(`/employer/applicants?job=${activeJob}`);
    cy.wait('@applications').its('request.query.job_id').should('eq', activeJob);
    cy.contains('No applicants for this job').should('be.visible');
    cy.get('form[role=search] select').should('have.value', activeJob);
  });

  it('sends the old Applicants address to the new page and to login when the session expires', () => {
    login();
    stubs();
    cy.intercept('GET', `${gateway}/v1/applications?*`, { statusCode: 401, body: { code: 16, message: 'expired' } }).as('expired');
    visit('/employer/applications');
    cy.wait('@expired');
    cy.location('pathname').should('eq', '/login');
  });
});

describe('JT-111 application page', () => {
  const applicationId = '99999999-9999-4999-8999-999999999999';
  const jobId = '22222222-2222-4222-8222-222222222222';
  const base = `${gateway}/v1/applications/${applicationId}`;
  const path = `/employer/applicants/${applicationId}`;
  const deadline = new Date(Date.now() + 10 * 86400000).toISOString();
  const job = { jobId, employerId, title: 'Data intern', description: 'Synthetic job', location: 'Thimphu', deadline, status: 'JOB_STATUS_PUBLISHED', timestamps };
  const application = (applicantName = 'Pema Choden', withdrawn = false) => ({ applicationId, jobId, applicantId: '11111111-1111-4111-8111-111111111111', employerId,
    resumeId: '88888888-8888-4888-8888-888888888888', applicantName, withdrawn, timestamps: { createdAt: '2026-11-20T04:00:00Z' } });
  const review = decision => ({ reviewId: '12121212-1212-4212-8212-121212121212', applicationId, score: 4, decision, timestamps: { createdAt: '2026-11-21T04:00:00Z' } });
  const offer = status => ({ offerId: '13131313-1313-4313-8313-131313131313', applicationId, terms: 'Synthetic terms', status });
  const notFound = { statusCode: 404, body: { code: 5, message: 'private' } };
  const history = [
    { previousStatus: 'APPLICATION_STATUS_UNSPECIFIED', newStatus: 'APPLICATION_STATUS_SUBMITTED', occurredAt: '2026-11-20T04:00:00Z' },
    { previousStatus: 'APPLICATION_STATUS_SUBMITTED', newStatus: 'APPLICATION_STATUS_UNDER_REVIEW', occurredAt: '2026-11-21T04:00:00Z' },
  ];
  const s = name => `APPLICATION_STATUS_${name}`;
  const pdfBase64 = Cypress.Buffer.from('%PDF-1.7\n').toString('base64');

  // Stubs one application in the given state. NOT_FOUND is the default for review, offer,
  // profile and photo, as for an application with none of them.
  function stub({ status = s('SUBMITTED'), app = application(), reviewBody = notFound, offerBody = notFound, profileBody = notFound, photoBody = notFound } = {}) {
    cy.intercept('GET', base, { application: app }).as('application');
    cy.intercept('GET', `${base}/status`, { statusRecord: { applicationId, status } }).as('status');
    cy.intercept('GET', `${base}/status-history?*`, { entries: history, pagination: { totalCount: history.length } }).as('history');
    cy.intercept('GET', `${base}/review`, reviewBody).as('review');
    cy.intercept('GET', `${base}/offer`, offerBody).as('offer');
    cy.intercept('GET', `${base}/interviews?*`, { interviews: [], pagination: { totalCount: 0 } }).as('interviews');
    cy.intercept('GET', `${base}/applicant-profile`, profileBody).as('profile');
    cy.intercept('GET', `${base}/applicant-photo`, photoBody).as('photo');
    cy.intercept('GET', `${base}/messages?*`, { messages: [], pagination: { totalCount: 0 } });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job });
  }
  function openPage(options) {
    login();
    stub(options);
    visit(path);
    // No wait on @status: a withdrawn application is final without reading it.
    cy.wait(['@application', '@review', '@offer']);
  }
  // Opens a dialog with its button and waits until it has fully faded in (MUI Fade animates the
  // container's opacity); before that Cypress counts everything inside as not visible.
  function openDialog(button) {
    cy.contains('main button', exact(button)).click();
    cy.get('[role=dialog]').should('be.visible').closest('.MuiDialog-container').should('have.css', 'opacity', '1');
  }
  const dialogAlert = text => cy.get('[role=dialog] [role=alert]').filter(`:contains("${text}")`).should('have.length', 1).and('be.visible');
  // Long text is set with the native setter and one input event (as in the JT-83 tests),
  // because typing it key by key is slow and trips React's update limit inside Cypress.
  const setDialogText = text => cy.get('[role=dialog]').then($d => {
    const el = $d.find('textarea')[0];
    const view = el.ownerDocument.defaultView;
    Object.getOwnPropertyDescriptor(view.HTMLTextAreaElement.prototype, 'value').set.call(el, text);
    el.dispatchEvent(new view.Event('input', { bubbles: true }));
  });
  const section = name => cy.get(`main section[aria-label="${name}"]`);
  const actions = ['Review application', 'Schedule interview', 'Schedule another interview', 'Make offer'];
  // On failure the message names the elements that cause the overflow: those whose right edge
  // passes the viewport while none of their children's does (the deepest ones), widest first.
  function overflowing(doc, limit) {
    const past = el => el.getBoundingClientRect().right > limit + 0.5;
    return [...doc.body.querySelectorAll('*')].filter(el => past(el) && ![...el.children].some(past))
      .map(el => {
        const r = el.getBoundingClientRect();
        const mui = [...el.classList].filter(c => c.startsWith('Mui')).slice(0, 2).join('.');
        const where = el.closest('section[aria-label], [role=dialog], header, nav')?.getAttribute('aria-label') ?? el.closest('header, nav')?.tagName.toLowerCase() ?? 'page';
        return { right: r.right, text: `${el.tagName.toLowerCase()}${mui ? `.${mui}` : ''} in "${where}" left ${Math.round(r.left)} right ${Math.round(r.right)} width ${Math.round(r.width)} "${(el.textContent ?? '').trim().slice(0, 40)}"` };
      })
      .sort((a, b) => b.right - a.right).slice(0, 5).map(o => o.text).join('; ');
  }
  const noHorizontalScroll = () => cy.document().then(doc => {
    const { scrollWidth, clientWidth } = doc.documentElement;
    expect(scrollWidth, scrollWidth > clientWidth ? `page width; overflowing: ${overflowing(doc, clientWidth)}` : 'page width').to.be.at.most(clientWidth);
  });

  it('shows the applicant, profile, job, status rail and history, with no IDs and no "from None"', () => {
    const profile = { applicantId: '11111111-1111-4111-8111-111111111111', summary: 'Synthetic summary', skills: ['Go'],
      education: [{ institution: 'Synthetic College', qualification: 'BSc', startYear: 2020, endYear: 2024 }] };
    openPage({ status: s('UNDER_REVIEW'), profileBody: { profile } });
    cy.contains('main a', 'Applicants').should('have.attr', 'href', '/employer/applicants');
    cy.contains('main h1', exact('Pema Choden')).should('be.visible');
    cy.contains('Data intern, applied 20 Nov').should('be.visible');
    section('Applicant').should('contain', 'Synthetic summary').and('contain', 'Synthetic College').and('contain', 'Go');
    section('Job').should('contain', 'Data intern').and('contain', 'Thimphu');
    section('Status').find('ol[aria-label^="Application progress"]').should('have.attr', 'aria-label', 'Application progress: Step 2 of 5, Under review, current');
    // JT-115 item 3 on this page: the first entry has no "from" part.
    cy.get('ul[aria-label="Status history"] > li').should('have.length', 2).first().invoke('text').should('match', /^Submitted/).and('not.contain', 'from');
    section('Status').should('not.contain', 'from None').and('contain', 'Under review from Submitted');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
  });

  it('has no horizontal scroll from 320px to desktop and keeps the back link below the top bar', () => {
    openPage({ status: s('INTERVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } });
    for (const [width, height] of [[320, 760], [390, 844], [768, 1024], [1280, 900], [1440, 900]]) {
      cy.viewport(width, height);
      cy.contains('main h1', 'Pema Choden').should('be.visible');
      noHorizontalScroll();
    }
    cy.get('header').then($bar => {
      cy.contains('main a', 'Applicants').then($back => {
        expect($back[0].getBoundingClientRect().top).to.be.at.least($bar[0].getBoundingClientRect().bottom);
      });
    });
  });

  it('opens at the top after scrolling the Applicants list, so the back link is not under the top bar', () => {
    login();
    stub();
    const apps = Array.from({ length: 8 }, (_, i) => ({ ...application(`Applicant ${i + 1}`), applicationId: i ? `9999999${i}-9999-4999-8999-999999999999` : applicationId }));
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: apps, pagination: { totalCount: apps.length } }).as('applications');
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, { jobs: [], pagination: { totalCount: 0 } });
    cy.intercept('GET', `${gateway}/v1/applications/*/status`, { statusRecord: { status: s('SUBMITTED') } });
    // Open Applicants with the desktop links, then narrow the window: under 900px the shell
    // replaces the desktop links with the Menu button (never both), so a link found just
    // before the resize would be removed while being clicked.
    open('Applicants');
    cy.wait('@applications');
    cy.contains('main h1', exact('Applicants')).should('be.visible');
    cy.get('ul[aria-label=Applicants] > li').should('have.length', apps.length);
    cy.viewport(390, 700);
    cy.get('button[aria-label="Open navigation"]').should('be.visible');
    cy.get('header nav[aria-label=Main]').should('not.exist');
    cy.window().then(win => win.scrollTo(0, win.document.documentElement.scrollHeight));
    cy.window().its('scrollY').should('be.greaterThan', 0);
    cy.contains('ul[aria-label=Applicants] > li', 'Applicant 1').contains('a', 'View').click();
    cy.location('pathname').should('eq', path);
    cy.window().its('scrollY').should('eq', 0);
    cy.get('header').then($bar => {
      cy.contains('main a', 'Applicants').then($back => {
        expect($back[0].getBoundingClientRect().top).to.be.at.least($bar[0].getBoundingClientRect().bottom);
      });
    });
  });

  // Redesign doc Section 8: which actions each state offers.
  const lifecycle = [
    ['Submitted, not reviewed', { status: s('SUBMITTED') }, ['Review application'], true],
    ['Under review, not reviewed', { status: s('UNDER_REVIEW') }, ['Review application'], true],
    ['Under review, already reviewed', { status: s('UNDER_REVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } }, [], true],
    ['Shortlisted', { status: s('SHORTLISTED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } }, ['Schedule interview'], true],
    ['Interview, no offer', { status: s('INTERVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } }, ['Schedule another interview', 'Make offer'], true],
    ['Interview, offer exists', { status: s('INTERVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') }, offerBody: { offer: offer('OFFER_STATUS_PENDING') } },
      ['Schedule another interview'], true],
    ['Offered', { status: s('OFFERED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') }, offerBody: { offer: offer('OFFER_STATUS_PENDING') } }, [], true],
    ['Accepted', { status: s('ACCEPTED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') }, offerBody: { offer: offer('OFFER_STATUS_ACCEPTED') } }, [], false],
    ['Rejected', { status: s('REJECTED'), reviewBody: { review: review('REVIEW_DECISION_REJECTED') } }, [], false],
    ['Withdrawn', { status: s('SUBMITTED'), app: application('Pema Choden', true) }, [], false],
  ];
  for (const [name, options, shown, canMessage] of lifecycle) {
    it(`offers ${shown.length ? shown.join(' and ') : 'no actions'} when ${name}`, () => {
      openPage(options);
      cy.contains('main h1', 'Pema Choden').should('be.visible');
      section('Status').find('ol[aria-label^="Application progress"]').should('exist');
      for (const label of actions) cy.contains('main button', exact(label)).should(shown.includes(label) ? 'be.visible' : 'not.exist');
      if (canMessage) section('Conversation').contains('button', 'Send message').should('exist');
      else section('Conversation').should('contain', 'This application has ended, so the conversation is view only.').find('textarea').should('not.exist');
    });
  }

  it('reviews after confirmation, shows the review and reads the status again', () => {
    openPage({ status: s('SUBMITTED') });
    openDialog('Review application');
    cy.get('[role=dialog]').within(() => {
      cy.contains('button', 'Save review').should('be.disabled');
      cy.contains('label', 'Score').parent().find('[role=combobox]').click();
    });
    cy.contains('[role=option]', '4 of 5').click();
    cy.intercept('POST', `${base}/reviews`, req => {
      expect(req.body).to.deep.eq({ score: 4, decision: 'REVIEW_DECISION_SHORTLISTED' });
      req.reply({ review: review('REVIEW_DECISION_SHORTLISTED') });
    }).as('complete');
    cy.get('[role=dialog]').within(() => {
      cy.contains('label', 'Shortlist').click();
      cy.contains('button', 'Shortlist applicant').click();
    });
    cy.wait('@complete');
    cy.get('[role=dialog]').should('not.exist');
    cy.get('main [role=status]').filter(':contains("Review saved")').should('have.length', 1).and('contain', 'The applicant is shortlisted.');
    section('Review').should('contain', 'Score 4 of 5');
    cy.contains('button', 'Review application').should('not.exist');
    // Status is read again after the action (events update it a moment later).
    cy.get('@status.all').should('have.length.at.least', 2);
  });

  it('shows a fixed review error, never the server text', () => {
    openPage({ status: s('SUBMITTED') });
    cy.intercept('POST', `${base}/reviews`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    openDialog('Review application');
    cy.get('[role=dialog]').contains('label', 'Score').parent().find('[role=combobox]').click();
    cy.contains('[role=option]', '1 of 5').click();
    cy.get('[role=dialog]').contains('label', 'Reject').click();
    cy.get('[role=dialog]').contains('button', 'Reject application').click();
    dialogAlert('Only the employer who posted this job can review the application.').and('not.contain', 'private-input');
  });

  it('schedules an interview in UTC from local time and blocks a past time and 1001 characters', () => {
    openPage({ status: s('SHORTLISTED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } });
    openDialog('Schedule interview');
    const local = date => new Date(date.getTime() - date.getTimezoneOffset() * 60000).toISOString().slice(0, 16);
    const past = new Date(Date.now() - 3600000);
    const future = new Date(Date.now() + 2 * 86400000); future.setSeconds(0, 0);
    cy.get('[role=dialog]').within(() => field('Date and time').type(local(past)));
    dialogAlert('Choose a time in the future.');
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).should('be.disabled');
    cy.get('[role=dialog]').within(() => field('Date and time').clear().type(local(future)));
    cy.get('[role=dialog] [role=alert]').should('not.exist');
    setDialogText('x'.repeat(1001));
    dialogAlert('Details can have at most 1000 characters.');
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).should('be.disabled');
    setDialogText('x'.repeat(1000));
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).should('be.enabled');
    setDialogText(' Room 2 ');
    cy.intercept('POST', `${base}/interviews`, req => {
      expect(req.body).to.deep.eq({ scheduledAt: future.toISOString(), details: 'Room 2' });
      req.reply({ interview: { interviewId: '14141414-1414-4414-8414-141414141414', applicationId, scheduledAt: future.toISOString(), details: 'Room 2' } });
    }).as('schedule');
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).click();
    cy.wait('@schedule');
    cy.contains('main [role=status]', 'Interview scheduled for').should('be.visible');
    cy.wait('@interviews');
  });

  it('makes an offer with 1 to 2000 characters and shows a duplicate offer with a fixed message', () => {
    openPage({ status: s('INTERVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } });
    openDialog('Make offer');
    cy.get('[role=dialog]').contains('button', 'Send offer').should('be.disabled');
    setDialogText('x'.repeat(2001));
    cy.get('[role=dialog]').contains('button', 'Send offer').should('be.disabled');
    dialogAlert('Offer terms can have at most 2000 characters.');
    setDialogText(` ${'y'.repeat(2000)} `);
    let calls = 0;
    cy.intercept('POST', `${base}/offers`, req => {
      expect(req.body).to.deep.eq({ terms: 'y'.repeat(2000) });
      calls += 1;
      if (calls === 1) req.reply({ statusCode: 409, body: { code: 6, message: 'private-input' } });
      else req.reply({ offer: { ...offer('OFFER_STATUS_PENDING'), terms: req.body.terms } });
    }).as('make');
    cy.get('[role=dialog]').contains('button', 'Send offer').click();
    cy.wait('@make');
    dialogAlert('An offer already exists for this application.').and('not.contain', 'private-input');
    cy.get('[role=dialog]').contains('button', 'Send offer').click();
    cy.wait('@make');
    cy.contains('main [role=status]', 'Offer sent.').should('be.visible');
    section('Offer').should('contain', 'Awaiting reply');
    cy.contains('button', 'Make offer').should('not.exist');
  });

  it('shows short empty states for NOT_FOUND profile, photo, resume, review and offer, and "Applicant" without a name', () => {
    openPage({ status: s('SHORTLISTED'), app: application('') });
    cy.contains('main h1', exact('Applicant')).should('be.visible');
    section('Applicant').should('contain', 'No profile is available for this application.').find('img').should('not.exist');
    section('Review').should('contain', 'No review was recorded for this application.');
    section('Offer').should('contain', 'No offer yet.');
    cy.get('main [role=alert]').should('not.exist');
    cy.window().then(win => cy.stub(win, 'open').returns(null));
    cy.intercept('GET', `${base}/resume`, notFound).as('resume');
    section('Applicant').contains('button', 'Download resume').click();
    cy.wait('@resume');
    section('Applicant').should('contain', 'No resume is available for this application.');
    cy.get('main [role=alert]').should('not.exist');
  });

  it('opens the resume in a tab opened during the click, and shows the photo', () => {
    const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';
    openPage({ photoBody: { contentType: 'image/png', content: png } });
    section('Applicant').find('img').should('have.attr', 'src').and('match', /^blob:/);
    const fake = { opener: {}, closed: false, document: { title: '' }, location: { href: '' }, close: cy.stub() };
    cy.window().then(win => cy.stub(win, 'open').returns(fake).as('open'));
    cy.intercept('GET', `${base}/resume`, { fileName: 'cv.pdf', contentType: 'application/pdf', content: pdfBase64 }).as('resume');
    section('Applicant').contains('button', 'Download resume').click();
    cy.wait('@resume');
    cy.get('@open').should('have.been.calledOnceWith', '', '_blank');
    cy.wrap(fake).its('location.href').should('match', /^blob:/);
    cy.wrap(fake).its('opener').should('eq', null);
  });

  it('sends the old employer addresses to the application page or the right list', () => {
    login();
    stub();
    for (const old of [`/employer/applications/${applicationId}/status`, `/employer/applications/${applicationId}/review`,
      `/employer/applications/${applicationId}/interviews`, `/employer/offers/new/${applicationId}`,
      `/employer/offers?applicationId=${applicationId}`]) {
      visit(old);
      cy.location('pathname').should('eq', path);
    }
    // JT-113: the old messages address is now the inbox thread itself.
    visit(`/employer/messages/${applicationId}`);
    cy.location('pathname').should('eq', `/employer/messages/${applicationId}`);
    cy.contains('main h1', /^Messages$/).should('be.visible');
    for (const [old, target] of [['/employer/offers', '/employer/applicants'], ['/employer/report-job', '/employer/jobs'],
      [`/employer/report-job/${jobId}`, '/employer/jobs'], ['/employer/applicants/not-a-uuid', '/employer/applicants/not-a-uuid']]) {
      visit(old);
      cy.location('pathname').should('eq', target);
    }
    cy.contains('This application does not exist').should('be.visible');
  });

  // JT-83 status rules (moved from applicant-hiring-pages.cy.js).
  it('reads every history page, 20 at a time, oldest first', () => {
    login();
    stub({ status: s('SHORTLISTED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } });
    const third = { previousStatus: s('UNDER_REVIEW'), newStatus: s('SHORTLISTED'), occurredAt: '2026-11-22T04:00:00Z' };
    cy.intercept('GET', `${base}/status-history?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20' ? { entries: [third], pagination: { totalCount: 3 } }
        : { entries: history, pagination: { nextPageToken: '20', totalCount: 3 } });
    }).as('pages');
    visit(path);
    cy.get('ul[aria-label="Status history"] > li').should('have.length', 3).last().should('contain', 'Shortlisted from Under review');
    cy.get('@pages.all').should('have.length', 2);
  });

  it('explains a status read that is not allowed, once', () => {
    login();
    stub();
    cy.intercept('GET', `${base}/status`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    cy.intercept('GET', `${base}/status-history?*`, { statusCode: 403, body: { code: 7, message: 'private-input' } });
    visit(path);
    section('Status').find('[role=alert]').should('have.length', 1).and('contain', 'Only the applicant and the employer').and('not.contain', 'private-input');
  });

  it('treats a missing status record as Submitted and explains a missing history', () => {
    login();
    stub();
    cy.intercept('GET', `${base}/status`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    cy.intercept('GET', `${base}/status-history?*`, { statusCode: 404, body: { code: 5, message: 'private-input' } });
    visit(path);
    section('Status').find('ol[aria-label^="Application progress"]').should('have.attr', 'aria-label', 'Application progress: Step 1 of 5, Submitted, current');
    section('Status').find('[role=alert]').should('contain', 'has no status yet').and('not.contain', 'private-input');
  });

  it('rejects an application ID in the address that is not a UUID without calling the gateway', () => {
    let called = false;
    cy.intercept(`${gateway}/v1/applications/not-a-uuid**`, () => { called = true; });
    login();
    visit('/employer/applicants/not-a-uuid');
    cy.contains('This application does not exist').should('be.visible');
    cy.then(() => expect(called).to.eq(false));
  });

  // JT-83 review rules.
  it('sends no review when the dialog is cancelled', () => {
    openPage({ status: s('SUBMITTED') });
    cy.intercept('POST', `${base}/reviews`, () => { throw new Error('Cancel must not send a review'); });
    openDialog('Review application');
    cy.get('[role=dialog]').contains('button', 'Cancel').click();
    cy.get('[role=dialog]').should('not.exist');
    cy.contains('main button', 'Review application').should('be.visible');
  });

  it('shows an existing rejected review read-only, with its score and no Review button', () => {
    openPage({ status: s('REJECTED'), reviewBody: { review: { ...review('REVIEW_DECISION_REJECTED'), score: 2 } } });
    section('Review').should('contain', 'Rejected').and('contain', 'Score 2 of 5');
    cy.contains('main button', 'Review application').should('not.exist');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already been reviewed'], [404, 5, 'not available for review yet'], [403, 7, 'Only the employer who posted'], [400, 3, 'Choose a score from 1 to 5']]) {
    it(`explains a ${statusCode} answer to a review`, () => {
      openPage({ status: s('SUBMITTED') });
      cy.intercept('POST', `${base}/reviews`, { statusCode, body: { code, message: 'private-input' } });
      openDialog('Review application');
      cy.get('[role=dialog]').contains('label', 'Score').parent().find('[role=combobox]').click();
      cy.contains('[role=option]', '1 of 5').click();
      cy.get('[role=dialog]').contains('label', 'Reject').click();
      cy.get('[role=dialog]').contains('button', 'Reject application').click();
      dialogAlert(message).and('not.contain', 'private-input');
      cy.get('[role=dialog]').contains('button', 'Reject application').should('be.enabled');
    });
  }

  // JT-83 interview rules.
  const localInput = daysAhead => {
    const d = new Date(Date.now() + daysAhead * 86400000);
    d.setHours(10, 0, 0, 0);
    return { value: new Date(d.getTime() - d.getTimezoneOffset() * 60000).toISOString().slice(0, 16), iso: d.toISOString() };
  };
  const shortlisted = { status: s('SHORTLISTED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } };

  it('blocks a time more than 365 days ahead', () => {
    openPage(shortlisted);
    openDialog('Schedule interview');
    cy.get('[role=dialog]').within(() => field('Date and time').type(localInput(367).value));
    dialogAlert('within 365 days');
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).should('be.disabled');
  });

  it('lists the new interview after scheduling', () => {
    let scheduled = [];
    login();
    stub(shortlisted);
    cy.intercept('GET', `${base}/interviews?*`, req => req.reply({ interviews: scheduled, pagination: { totalCount: scheduled.length } })).as('list');
    visit(path);
    section('Interviews').should('contain', 'No interviews yet.');
    const when = localInput(3);
    cy.intercept('POST', `${base}/interviews`, req => {
      scheduled = [{ interviewId: '15151515-1515-4515-8515-151515151515', applicationId, scheduledAt: req.body.scheduledAt, details: 'Room 2\nBring your ID' }];
      req.reply({ interview: scheduled[0] });
    }).as('schedule');
    openDialog('Schedule interview');
    cy.get('[role=dialog]').within(() => field('Date and time').type(when.value));
    setDialogText('Room 2\nBring your ID');
    cy.get('[role=dialog]').contains('button', exact('Schedule interview')).click();
    cy.wait('@schedule').its('request.body.scheduledAt').should('eq', when.iso);
    section('Interviews').should('contain', 'Room 2').and('contain', 'Bring your ID');
  });

  for (const [statusCode, code, message] of [[409, 6, 'already scheduled at this time'], [400, 3, 'The interview was not scheduled'], [403, 7, 'Only the employer who posted'], [404, 5, 'not available yet']]) {
    it(`explains a ${statusCode} answer to Schedule interview`, () => {
      openPage(shortlisted);
      cy.intercept('POST', `${base}/interviews`, { statusCode, body: { code, message: 'private-input' } });
      openDialog('Schedule interview');
      cy.get('[role=dialog]').within(() => field('Date and time').type(localInput(2).value));
      cy.get('[role=dialog]').contains('button', exact('Schedule interview')).click();
      dialogAlert(message).and('not.contain', 'private-input');
      cy.get('[role=dialog]').contains('button', exact('Schedule interview')).should('be.enabled');
    });
  }

  // JT-78 messaging rules for the employer (the conversation is embedded in the page).
  const existing = { messageId: 'm1', senderId: '11111111-1111-4111-8111-111111111111', content: 'Existing message', timestamps: { createdAt: '2026-11-21T04:00:00Z' } };

  it('reads the thread and sends a trimmed message', () => {
    login();
    stub(shortlisted);
    cy.intercept('GET', `${base}/messages?*`, { messages: [existing], pagination: { totalCount: 1 } });
    visit(path);
    section('Conversation').find('[role=log]').should('contain', 'Existing message').and('contain', 'Pema Choden');
    section('Conversation').contains('button', 'Refresh messages').should('be.visible');
    section('Conversation').find('textarea').first().type('  Reply  ');
    cy.intercept('POST', `${base}/messages`, req => {
      expect(req.body).to.deep.eq({ content: 'Reply' });
      req.reply({ message: { messageId: 'm2', senderId: employerId, content: 'Reply', timestamps: { createdAt: '2026-11-22T04:00:00Z' } } });
    }).as('send');
    section('Conversation').contains('button', 'Send message').click();
    cy.wait('@send');
    section('Conversation').find('[role=log]').should('contain', 'Reply').and('contain', 'You');
    section('Conversation').find('textarea').first().should('have.value', '');
  });

  it('keeps the draft when sending fails and permits a retry', () => {
    login();
    stub(shortlisted);
    visit(path);
    section('Conversation').should('contain', 'No messages yet.');
    section('Conversation').contains('button', 'Send message').should('be.disabled');
    section('Conversation').find('textarea').first().type('Retry this draft');
    cy.intercept('POST', `${base}/messages`, { statusCode: 503, body: {} });
    section('Conversation').contains('button', 'Send message').click();
    section('Conversation').should('contain', 'The request failed');
    section('Conversation').find('textarea').first().should('have.value', 'Retry this draft');
    section('Conversation').contains('button', 'Send message').should('be.enabled');
  });

  it('blocks sending when the thread cannot be read', () => {
    login();
    stub(shortlisted);
    cy.intercept('GET', `${base}/messages?*`, { statusCode: 403, body: { code: 7 } });
    visit(path);
    section('Conversation').should('contain', 'You do not have permission');
    section('Conversation').find('textarea').first().type('Cannot send');
    section('Conversation').contains('button', 'Send message').should('be.disabled');
  });

  // JT-108 offer rules: the one lifetime offer is found by application, never by a typed ID.
  it('loads the offer by application, keeps it after a reload and gives the employer no Accept or Decline', () => {
    openPage({ status: s('OFFERED'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') }, offerBody: { offer: offer('OFFER_STATUS_PENDING') } });
    section('Offer').should('contain', 'Synthetic terms').and('contain', 'Awaiting reply');
    cy.contains('button', 'Accept offer').should('not.exist');
    cy.contains('button', 'Decline offer').should('not.exist');
    cy.reload();
    cy.location('pathname').should('eq', path);
    cy.wait('@offer');
    section('Offer').should('contain', 'Synthetic terms');
    cy.get('main').invoke('text').should('not.match', uuidPattern);
  });

  it('shows no offer separately from a failed read and allows retry', () => {
    openPage({ status: s('INTERVIEW'), reviewBody: { review: review('REVIEW_DECISION_SHORTLISTED') } });
    section('Offer').should('contain', 'No offer yet');
    cy.contains('main button', 'Make offer').should('be.visible');
    cy.intercept('GET', `${base}/offer`, { statusCode: 503, body: { code: 14, message: 'private-details' } }).as('failedOffer');
    section('Status').contains('button', exact('Refresh')).click();
    cy.wait('@failedOffer');
    section('Offer').find('[role=alert]').should('contain', 'The request failed').and('not.contain', 'private-details');
    section('Offer').should('not.contain', 'No offer yet');
    // A failed read is not "no offer", so Make offer is not offered either.
    cy.contains('main button', 'Make offer').should('not.exist');
    cy.intercept('GET', `${base}/offer`, { offer: offer('OFFER_STATUS_PENDING') });
    section('Offer').contains('button', 'Try again').click();
    section('Offer').should('contain', 'Synthetic terms');
  });

  it('ignores an old offer response after moving to another application', () => {
    const otherId = '98989898-9898-4898-8898-989898989898';
    const other = `${gateway}/v1/applications/${otherId}`;
    let release;
    const pending = new Promise(resolve => { release = resolve; });
    login();
    stub();
    cy.intercept('GET', `${base}/offer`, req => pending.then(() => req.reply({ offer: offer('OFFER_STATUS_PENDING') }))).as('oldOffer');
    cy.intercept('GET', other, { application: { ...application('Sonam Wangmo'), applicationId: otherId } });
    cy.intercept('GET', `${other}/status`, { statusRecord: { applicationId: otherId, status: s('OFFERED') } });
    cy.intercept('GET', `${other}/status-history?*`, { entries: history, pagination: {} });
    cy.intercept('GET', `${other}/offer`, { offer: { ...offer('OFFER_STATUS_PENDING'), applicationId: otherId, terms: 'Another application offer' } }).as('newOffer');
    for (const part of ['review', 'applicant-profile', 'applicant-photo']) cy.intercept('GET', `${other}/${part}`, notFound);
    cy.intercept('GET', `${other}/interviews?*`, { interviews: [], pagination: {} });
    cy.intercept('GET', `${other}/messages?*`, { messages: [], pagination: {} });
    visit(path);
    cy.contains('main h1', 'Pema Choden').should('be.visible');
    section('Offer').find('[role=status][aria-label="Loading offer"]').should('exist');
    visit(`/employer/applicants/${otherId}`);
    cy.wait('@newOffer');
    cy.contains('main h1', 'Sonam Wangmo').should('be.visible');
    section('Offer').should('contain', 'Another application offer');
    cy.then(() => release());
    cy.wait('@oldOffer');
    section('Offer').should('contain', 'Another application offer').and('not.contain', 'Synthetic terms');
  });
});
