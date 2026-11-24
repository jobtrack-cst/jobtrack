import { fill } from './fields';
import { settleHome } from './home';
// JT-113 Messages inbox (redesign doc Section 6), gateway-stubbed. These tests check the
// UI and the requests it sends, not live services. Run them with the Compose gateway
// stopped (redesign doc Section 9).
const gateway = 'http://localhost:8000';
const applicantId = '11111111-1111-4111-8111-111111111111';
const employerId = '66666666-6666-4666-8666-666666666666';
const uuid = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
// A 1x1 PNG.
const png = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==';
const notFound = { statusCode: 404, body: { code: 5, message: 'private' } };
const exact = text => new RegExp(`^${Cypress._.escapeRegExp(text)}$`);
const open = link => cy.contains('nav a', exact(link)).click();
function visit(path) {
  cy.window().then(win => { win.history.pushState({}, '', path); win.dispatchEvent(new PopStateEvent('popstate')); });
}
const rows = () => cy.get('ul[aria-label=Conversations] > li');
const row = text => cy.contains('ul[aria-label=Conversations] > li', text);
const thread = name => cy.get(`section[aria-label="Conversation with ${name}"]`);
const noHorizontalScroll = () => cy.document().then(doc => expect(doc.documentElement.scrollWidth).to.be.at.most(doc.documentElement.clientWidth));
const idOf = n => `a0000000-0000-4000-8000-${String(n).padStart(12, '0')}`;

function login(role) {
  const user = role === 'ROLE_EMPLOYER'
    ? { userId: employerId, email: 'dorji@example.test', fullName: 'Dorji', role }
    : { userId: applicantId, email: 'pema@example.test', fullName: 'Pema Choden', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt113-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user }).as('login');
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt113-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type(user.email);
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.wait('@login');
  cy.location('pathname').should('eq', role === 'ROLE_EMPLOYER' ? '/employer' : '/applicant');
  settleHome();
}

const message = (n, senderId, content, createdAt) => ({ messageId: `m${n}`, senderId, content, timestamps: { createdAt } });

// Stubs ListApplications (paged by 20), ListMessages (paged by 20, oldest first; 'notFound'
// or 'fail' instead of a list), SendMessage (added to the thread), GetApplicationStatus and GetJob.
function stubInbox({ apps, threads = {}, statuses = {}, jobs = [], selfId = applicantId }) {
  cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
    const offset = Number(req.query['pagination.page_token'] || 0);
    const page = apps.slice(offset, offset + 20);
    req.reply({ applications: page, pagination: { totalCount: apps.length, nextPageToken: offset + 20 < apps.length ? String(offset + 20) : '' } });
  }).as('applications');
  cy.intercept('GET', `${gateway}/v1/applications/*/messages?*`, req => {
    const id = req.url.split('/applications/')[1].split('/')[0];
    const list = threads[id] ?? [];
    if (list === 'notFound') return req.reply(notFound);
    if (list === 'fail') return req.reply({ statusCode: 500, body: { code: 13, message: 'private' } });
    const offset = Number(req.query['pagination.page_token'] || 0);
    req.reply({ messages: list.slice(offset, offset + 20), pagination: { totalCount: list.length, nextPageToken: offset + 20 < list.length ? String(offset + 20) : '' } });
  }).as('messages');
  cy.intercept('POST', `${gateway}/v1/applications/*/messages`, req => {
    const id = req.url.split('/applications/')[1].split('/')[0];
    const sent = message(`sent-${Date.now()}`, selfId, req.body.content, new Date().toISOString());
    threads[id] = Array.isArray(threads[id]) ? [...threads[id], sent] : [sent];
    req.reply({ message: sent });
  }).as('send');
  cy.intercept('GET', `${gateway}/v1/applications/*/status`, req => {
    const id = req.url.split('/applications/')[1].split('/')[0];
    const status = statuses[id];
    if (!status) return req.reply(notFound);
    req.reply({ statusRecord: { applicationId: id, status } });
  }).as('status');
  for (const job of jobs) cy.intercept('GET', `${gateway}/v1/jobs/${job.jobId}`, { job });
}

describe('JT-113 messages inbox for applicants', () => {
  const companies = [
    { employerId: '61000000-0000-4000-8000-000000000001', companyName: 'Druk Tech', hasLogo: true },
    { employerId: '61000000-0000-4000-8000-000000000002', companyName: 'Bhutan Data', hasLogo: false },
    // The profile says there is a logo, but the logo read is NOT_FOUND: initials, no error.
    { employerId: '61000000-0000-4000-8000-000000000003', companyName: 'Paro Labs', hasLogo: true },
  ];
  const jobOf = (n, title, employer) => ({ jobId: `22000000-0000-4000-8000-00000000000${n}`, employerId: employer.employerId, title,
    description: 'Synthetic job', location: 'Thimphu', deadline: new Date(Date.now() + 10 * 86400000).toISOString(), status: 'JOB_STATUS_PUBLISHED' });
  const jobs = [jobOf(1, 'Frontend intern', companies[0]), jobOf(2, 'Data intern', companies[1]), jobOf(3, 'QA intern', companies[2]), jobOf(4, 'Design intern', companies[1])];
  const app = (n, job, createdAt, extra = {}) => ({ applicationId: idOf(n), jobId: job.jobId, applicantId, employerId: job.employerId,
    resumeId: '30000000-0000-4000-8000-000000000001', applicantName: 'Pema Choden', timestamps: { createdAt }, ...extra });
  const [a, b, c, d] = [app(1, jobs[0], '2026-11-14T04:00:00Z'), app(2, jobs[1], '2026-11-18T04:00:00Z'), app(3, jobs[2], '2026-11-20T04:00:00Z'), app(4, jobs[3], '2026-11-10T04:00:00Z')];
  const threads = () => ({
    [a.applicationId]: [message(1, applicantId, 'Hello', '2026-11-21T04:00:00Z'), message(2, companies[0].employerId, 'Please share your\nportfolio', '2026-11-22T04:00:00Z')],
    [b.applicationId]: [],
    // Messaging has not processed this application yet.
    [c.applicationId]: 'notFound',
    [d.applicationId]: [message(3, applicantId, 'Thank you for the interview', '2026-11-21T03:00:00Z')],
  });
  const statuses = { [a.applicationId]: 'APPLICATION_STATUS_SHORTLISTED', [d.applicationId]: 'APPLICATION_STATUS_INTERVIEW' };

  function stubCompanies() {
    for (const profile of companies) cy.intercept('GET', `${gateway}/v1/companies/${profile.employerId}`, { profile }).as(profile.companyName.replace(/\s/g, ''));
    cy.intercept('GET', `${gateway}/v1/companies/${companies[0].employerId}/logo`, { contentType: 'image/png', content: png });
    cy.intercept('GET', `${gateway}/v1/companies/${companies[2].employerId}/logo`, notFound);
  }
  function openInbox(options = {}) {
    login('ROLE_APPLICANT');
    stubCompanies();
    stubInbox({ apps: [a, b, c, d], threads: threads(), statuses, jobs, ...options });
    open('Messages');
    cy.location('pathname').should('eq', '/applicant/messages');
  }

  it('adds Messages after Applications and lists conversations by newest activity, with no IDs', () => {
    openInbox();
    cy.get('nav[aria-label=Main] a').then($links => expect([...$links].map(l => l.textContent.trim()))
      .to.deep.eq(['Home', 'Find jobs', 'Saved', 'Applications', 'Messages', 'Profile', 'Job alerts']));
    cy.contains('main h2', exact('4 conversations')).should('be.visible');
    // Last message 22 Nov, last message 21 Nov, then no messages by applied date (20 Nov, 18 Nov).
    // Retried: each row shows "Employer" until its company profile arrives.
    rows().should($rows => expect([...$rows].map(r => r.querySelector('p').textContent))
      .to.deep.eq(['Druk Tech', 'Bhutan Data', 'Paro Labs', 'Bhutan Data']));
    row('Frontend intern').within(() => {
      cy.contains('Please share your portfolio').should('be.visible');
      cy.contains('22 Nov').should('be.visible');
      cy.get('img').should('have.attr', 'src').and('match', /^data:image\/png;base64,/);
    });
    row('Design intern').contains('You: Thank you for the interview').should('be.visible');
    row('Data intern').contains('No messages yet').should('be.visible');
    row('QA intern').within(() => {
      cy.contains('No messages yet').should('be.visible');
      cy.get('img').should('not.exist');
      cy.contains(exact('PL')).should('be.visible');
    });
    cy.contains('Choose a conversation').should('be.visible');
    cy.get('[role=alert]').should('not.exist');
    cy.get('main').invoke('text').should('not.match', uuid);
  });

  it('opens a thread beside the list, keeps it in the address on reload, and Back returns to the previous one', () => {
    openInbox();
    row('Frontend intern').click();
    cy.location('pathname').should('eq', `/applicant/messages/${a.applicationId}`);
    row('Frontend intern').find('a').should('have.attr', 'aria-current', 'true');
    thread('Druk Tech').within(() => {
      cy.contains('Frontend intern').should('be.visible');
      cy.contains('Shortlisted').should('be.visible');
      cy.contains('a', 'View application').should('have.attr', 'href', `/applicant/applications/${a.applicationId}`);
      cy.get('[role=log]').within(() => {
        cy.contains('You').should('be.visible');
        cy.contains('Please share your').should('be.visible');
      });
      cy.contains('button', 'Send message').should('exist');
    });
    rows().should('have.length', 4);
    // The profile is read once per page load (cached); take this load's read, so the wait
    // after the reload is for the reloaded page's own read.
    cy.wait('@DrukTech');
    cy.reload();
    cy.location('pathname').should('eq', `/applicant/messages/${a.applicationId}`);
    cy.wait('@DrukTech');
    thread('Druk Tech').contains('Please share your').should('be.visible');
    row('Design intern').click();
    thread('Bhutan Data').contains('Thank you for the interview').should('be.visible');
    cy.go('back');
    cy.location('pathname').should('eq', `/applicant/messages/${a.applicationId}`);
    thread('Druk Tech').should('be.visible');
    cy.get('main').invoke('text').should('not.match', uuid);
  });

  it('sends a message within 1 to 2000 characters and moves the conversation to the top', () => {
    openInbox();
    row('Data intern').click();
    thread('Bhutan Data').within(() => {
      cy.contains('No messages yet. Write to Bhutan Data below.').should('be.visible');
      fill('Message', 'x'.repeat(2001));
      cy.contains('2001/2000 characters').should('be.visible');
      cy.contains('button', 'Send message').should('be.disabled');
      fill('Message', 'x'.repeat(2000));
      cy.contains('2000/2000 characters').should('be.visible');
      cy.contains('button', 'Send message').should('be.enabled');
      fill('Message', 'Thanks, I am available on Monday');
      cy.contains('button', 'Send message').click();
    });
    cy.wait('@send').its('request.body').should('deep.eq', { content: 'Thanks, I am available on Monday' });
    rows().first().should('contain', 'Data intern').and('contain', 'You: Thanks, I am available on Monday');
    thread('Bhutan Data').find('[role=log]').contains('Thanks, I am available on Monday').should('be.visible');
  });

  it('makes a final application view only', () => {
    const withdrawn = { ...a, withdrawn: true };
    login('ROLE_APPLICANT');
    stubCompanies();
    stubInbox({ apps: [withdrawn], threads: threads(), statuses: {}, jobs });
    visit(`/applicant/messages/${a.applicationId}`);
    thread('Druk Tech').within(() => {
      cy.contains('Withdrawn').should('be.visible');
      cy.contains('This application has ended, so the conversation is view only.').should('be.visible');
      cy.contains('button', 'Send message').should('not.exist');
    });
  });

  it('reads the last message of a long thread at offset totalCount - 1, and marks a failed read on that row only', () => {
    const long = Array.from({ length: 23 }, (_, i) => message(100 + i, companies[0].employerId, `Message ${i + 1}`, `2026-11-22T04:${String(i).padStart(2, '0')}:00Z`));
    openInbox({ threads: { ...threads(), [a.applicationId]: long, [d.applicationId]: 'fail' } });
    row('Frontend intern').contains('Message 23').should('be.visible');
    cy.get('@messages.all').then(calls => {
      const tokens = calls.filter(call => call.request.url.includes(a.applicationId)).map(call => call.request.query['pagination.page_token']);
      expect(tokens).to.deep.eq(['', '22']);
    });
    row('Design intern').contains('Last message unavailable').should('be.visible');
    row('Data intern').contains('No messages yet').should('be.visible');
    cy.get('[role=alert]').should('not.exist');
  });

  it('pages more than 20 conversations in the browser', () => {
    const many = Array.from({ length: 21 }, (_, i) => app(50 + i, jobs[1], `2026-11-${String(i + 1).padStart(2, '0')}T04:00:00Z`));
    openInbox({ apps: many, threads: {} });
    cy.get('@applications.all').should('have.length', 2);
    cy.contains('main h2', exact('21 conversations')).should('be.visible');
    rows().should('have.length', 20);
    cy.contains('nav[aria-label=Pages]', 'Showing 1 to 20 of 21').contains('button', exact('Next')).click();
    cy.contains('nav[aria-label=Pages]', 'Showing 21 to 21 of 21').should('be.visible');
    rows().should('have.length', 1);
    cy.get('@applications.all').should('have.length', 2);
  });

  it('says when a conversation is not available and sends an invalid address to the list', () => {
    openInbox();
    visit(`/applicant/messages/${idOf(999)}`);
    cy.contains('This conversation is not available').should('be.visible');
    cy.contains('a', 'All messages').click();
    cy.location('pathname').should('eq', '/applicant/messages');
    visit('/applicant/messages/not-an-id');
    cy.location('pathname').should('eq', '/applicant/messages');
    cy.get('main').invoke('text').should('not.match', uuid);
  });

  it('shows the list and the thread as separate screens on phones, with no horizontal scroll', () => {
    cy.viewport(320, 800);
    login('ROLE_APPLICANT');
    stubCompanies();
    stubInbox({ apps: [a, b, c, d], threads: threads(), statuses, jobs });
    visit('/applicant/messages');
    rows().should('have.length', 4);
    cy.contains('Choose a conversation').should('not.exist');
    noHorizontalScroll();
    row('Frontend intern').click();
    cy.location('pathname').should('eq', `/applicant/messages/${a.applicationId}`);
    cy.get('ul[aria-label=Conversations]').should('not.exist');
    thread('Druk Tech').contains('button', 'Send message').scrollIntoView().should('be.visible');
    noHorizontalScroll();
    cy.contains('a', 'All messages').click();
    cy.location('pathname').should('eq', '/applicant/messages');
    rows().should('have.length', 4);
  });

  it('shows the list and the thread side by side from 900px, with no horizontal scroll', () => {
    cy.viewport(900, 900);
    openInbox();
    row('Frontend intern').click();
    cy.get('ul[aria-label=Conversations]').should('be.visible');
    thread('Druk Tech').should('be.visible').then($thread => {
      cy.get('ul[aria-label=Conversations]').then($list => expect($thread[0].getBoundingClientRect().left).to.be.greaterThan($list[0].getBoundingClientRect().right));
    });
    noHorizontalScroll();
  });
});

describe('JT-113 messages inbox for employers', () => {
  const activeJob = '22222222-2222-4222-8222-222222222222';
  const closedJob = '33333333-3333-4333-8333-333333333333';
  const removedJob = '44444444-4444-4444-8444-444444444444';
  const deadline = new Date(Date.now() + 10 * 86400000).toISOString();
  const jobOf = (jobId, title, status) => ({ jobId, employerId, title, description: 'Synthetic job', location: 'Thimphu', deadline, status });
  const jobs = {
    JOB_STATUS_PUBLISHED: [jobOf(activeJob, 'Data intern', 'JOB_STATUS_PUBLISHED')],
    JOB_STATUS_CLOSED: [jobOf(closedJob, 'Web intern', 'JOB_STATUS_CLOSED')],
    JOB_STATUS_REMOVED: [jobOf(removedJob, 'Old intern', 'JOB_STATUS_REMOVED')],
  };
  const app = (n, jobId, applicantName, createdAt) => ({ applicationId: idOf(n), jobId, applicantId: `11000000-0000-4000-8000-00000000000${n}`, employerId,
    resumeId: '88888888-8888-4888-8888-888888888888', applicantName, timestamps: { createdAt } });
  const named = app(1, activeJob, 'Pema Choden', '2026-11-18T04:00:00Z');
  // Submitted before JT-114: no name and no photo.
  const older = app(2, closedJob, '', '2026-11-20T04:00:00Z');

  function openInbox() {
    login('ROLE_EMPLOYER');
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, req => {
      const list = jobs[req.query.status] ?? [];
      req.reply({ jobs: list, pagination: { totalCount: list.length } });
    }).as('jobs');
    cy.intercept('GET', `${gateway}/v1/applications/${named.applicationId}/applicant-photo`, { contentType: 'image/png', content: png }).as('photo');
    cy.intercept('GET', `${gateway}/v1/applications/${older.applicationId}/applicant-photo`, notFound).as('noPhoto');
    stubInbox({
      apps: [named, older],
      threads: { [named.applicationId]: [message(1, named.applicantId, 'I have attached my resume', '2026-11-21T04:00:00Z')], [older.applicationId]: [] },
      statuses: { [named.applicationId]: 'APPLICATION_STATUS_UNDER_REVIEW', [older.applicationId]: 'APPLICATION_STATUS_SUBMITTED' },
      selfId: employerId,
    });
    cy.window().then(win => cy.spy(win.URL, 'revokeObjectURL').as('revoke'));
    open('Messages');
    cy.location('pathname').should('eq', '/employer/messages');
  }

  it('adds Messages after Applicants and shows applicant names, photos or initials, with no IDs', () => {
    openInbox();
    cy.get('nav[aria-label=Main] a').then($links => expect([...$links].map(l => l.textContent.trim()))
      .to.deep.eq(['Home', 'Jobs', 'Applicants', 'Messages', 'Company']));
    cy.wait(['@photo', '@noPhoto']);
    rows().should('have.length', 2).first().should('contain', 'Pema Choden');
    row('Pema Choden').within(() => {
      cy.contains('Data intern').should('be.visible');
      cy.contains('I have attached my resume').should('be.visible');
      cy.get('img').should('have.attr', 'src').and('match', /^blob:/);
    });
    row('Web intern').within(() => {
      cy.contains(exact('Applicant')).should('be.visible');
      cy.contains('No messages yet').should('be.visible');
      cy.get('img').should('not.exist');
      cy.contains(exact('A')).should('be.visible');
    });
    cy.get('[role=alert]').should('not.exist');
    cy.get('main').invoke('text').should('not.match', uuid);
    // Photo object URLs are revoked when the page unmounts.
    row('Pema Choden').find('img').invoke('attr', 'src').then(src => {
      open('Jobs');
      cy.get('@revoke').should('have.been.calledWith', src);
    });
  });

  it('filters by job in the browser, keeps ?job= in the thread address, and opens the thread', () => {
    openInbox();
    rows().should('have.length', 2);
    cy.get('form[role=search] select option').should($options => {
      expect([...$options].map(o => o.textContent)).to.deep.eq(['All jobs', 'Data intern', 'Web intern (Closed)', 'Old intern (Removed)']);
    });
    cy.get('form[role=search] select').select('Web intern (Closed)');
    cy.get('form[role=search]').contains('button', 'Show conversations').click();
    cy.location('search').should('eq', `?job=${closedJob}`);
    cy.contains('main h2', exact('1 conversation')).should('be.visible');
    cy.contains('For Web intern (Closed)').should('be.visible');
    rows().should('have.length', 1).first().should('contain', 'Web intern');
    // The filter reads nothing again.
    cy.get('@applications.all').should('have.length', 1);
    row('Web intern').click();
    cy.location('pathname').should('eq', `/employer/messages/${older.applicationId}`);
    cy.location('search').should('eq', `?job=${closedJob}`);
    thread('Applicant').within(() => {
      cy.contains('Submitted').should('be.visible');
      cy.contains('a', 'View application').should('have.attr', 'href', `/employer/applicants/${older.applicationId}`);
      cy.contains('No messages yet. Write to Applicant below.').should('be.visible');
    });
    cy.reload();
    cy.location('search').should('eq', `?job=${closedJob}`);
    thread('Applicant').should('be.visible');
    cy.get('form[role=search] select').should('have.value', closedJob);
    cy.get('main').invoke('text').should('not.match', uuid);
  });
});
