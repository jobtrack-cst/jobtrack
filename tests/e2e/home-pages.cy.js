// JT-110 home pages and the two optional list filters, gateway-stubbed.
import { settleHome } from './home';
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
const employerId = '66666666-6666-4666-8666-666666666666';
const jobId = '22222222-2222-4222-8222-222222222222';
const otherJobId = '33333333-3333-4333-8333-333333333333';
const uuid = /[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}/i;
const day = 86_400_000;
const inDays = days => new Date(Date.now() + days * day).toISOString();
const job = (id, title, deadline = inDays(30)) => ({ jobId: id, employerId, title, description: 'Synthetic job', status: 'JOB_STATUS_PUBLISHED', deadline, location: 'Thimphu' });
const application = (id, forJob, createdAt = '2026-11-12T04:00:00Z') => ({ applicationId: id, jobId: forJob, applicantId: userId, employerId, resumeId: '44444444-4444-4444-8444-444444444444', timestamps: { createdAt } });

function login(role) {
  const user = { userId, fullName: 'Pema Choden', email: 'pema@example.test', role };
  // Registered first, so the specific stubs in each test take precedence.
  cy.intercept('GET', `${gateway}/v1/**`, { body: {} });
  cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: { code: 5 } });
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt110-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  cy.intercept('GET', `${gateway}/v1/companies/${employerId}`, { profile: { employerId, companyName: 'Druk Tech' } });
}
function signIn(landing) {
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', landing);
}
const noUuidOnPage = () => cy.get('main').invoke('text').should('not.match', uuid);

describe('JT-110 optional list filters', () => {
  it('employer home asks for published jobs and counts applications per job with job_id', () => {
    login('ROLE_EMPLOYER');
    cy.intercept('GET', `${gateway}/v1/employer/profile`, { profile: { employerId: userId, companyName: 'Druk Tech', verificationStatus: 'VERIFICATION_STATUS_VERIFIED' } });
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, req => {
      expect(req.query.status).to.eq('JOB_STATUS_PUBLISHED');
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply({ jobs: [job(jobId, 'Frontend intern'), job(otherJobId, 'Data intern', inDays(3))], pagination: { totalCount: 2 } });
    }).as('jobs');
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      const forJob = req.query.job_id;
      if (forJob === undefined) req.reply({ applications: [application('55555555-5555-4555-8555-555555555555', jobId)], pagination: { totalCount: 1 } });
      else req.reply({ applications: [], pagination: { totalCount: forJob === jobId ? 2 : 1 } });
    }).as('applications');
    cy.intercept('GET', `${gateway}/v1/applications/*/status`, { statusRecord: { status: 'APPLICATION_STATUS_UNDER_REVIEW' } });
    signIn('/employer');
    cy.wait('@jobs');
    cy.contains('li', 'Frontend intern').should('contain', '2 applications');
    cy.contains('li', 'Data intern').should('contain', '1 application').and('contain', 'Closes in 3 days');
    cy.get('@applications.all').then(calls => {
      const jobIds = calls.map(c => c.request.query.job_id);
      expect(jobIds.filter(id => id !== undefined).sort()).to.deep.eq([jobId, otherJobId]);
      // The newest-applicants list is the one call without a filter.
      expect(jobIds.filter(id => id === undefined)).to.have.length(1);
    });
    // JT-111 rows: photo or initials, the name ("Applicant" before JT-114), job and date, status; the row opens the application page.
    cy.contains('section[aria-label="Newest applicants"] li', 'Frontend intern, applied 12 Nov').within(() => {
      cy.contains('h3', /^Applicant$/).should('be.visible');
      cy.contains('Under review').should('be.visible');
      cy.get('a').should('have.attr', 'href', '/employer/applicants/55555555-5555-4555-8555-555555555555');
    });
    cy.contains('Your company is verified').should('be.visible');
    noUuidOnPage();
  });

  it('sends each Jobs tab its own status, and no job_id when no job is chosen', () => {
    login('ROLE_EMPLOYER');
    signIn('/employer');
    settleHome();
    // JT-111: Jobs has Active, Drafts and Closed tabs, each using the ListMyJobs status filter.
    cy.intercept('GET', `${gateway}/v1/employer/jobs?*`, { jobs: [], pagination: { totalCount: 0 } }).as('jobs');
    cy.contains('nav a', /^Jobs$/).click();
    cy.wait('@jobs').its('request.query.status').should('eq', 'JOB_STATUS_PUBLISHED');
    cy.contains('[role=tab]', /^Drafts$/).click();
    cy.wait('@jobs').its('request.query.status').should('eq', 'JOB_STATUS_DRAFT');
    // Closed lists closed and removed jobs, one request per status.
    cy.contains('[role=tab]', /^Closed$/).click();
    cy.wait(['@jobs', '@jobs']).then(calls => {
      expect(calls.map(c => c.request.query.status).sort()).to.deep.eq(['JOB_STATUS_CLOSED', 'JOB_STATUS_REMOVED']);
    });
    // No Jobs request is ever sent without a status.
    cy.get('@jobs.all').each(call => expect(call.request.query).to.have.property('status'));
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query).not.to.have.property('job_id');
      req.reply({ applications: [], pagination: {} });
    }).as('allApplications');
    cy.contains('nav a', /^Applicants$/).click();
    cy.wait('@allApplications').its('request.url').should('not.contain', 'job_id=');
  });
});

describe('JT-110 applicant home', () => {
  it('shows active applications with the status rail, saved jobs closing soon and no IDs', () => {
    login('ROLE_APPLICANT');
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { applicantId: userId, summary: 'Synthetic' } });
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [{ resumeId: 'r1', fileName: 'cv.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_VALIDATED' }], pagination: {} });
    const active = application('55555555-5555-4555-8555-555555555555', jobId, '2026-11-14T04:00:00Z');
    const rejected = application('55555555-5555-4555-8555-555555555556', otherJobId);
    // An older application for the same job as the active one must not add a second card.
    const older = application('55555555-5555-4555-8555-555555555557', jobId, '2026-11-01T04:00:00Z');
    cy.intercept('GET', `${gateway}/v1/applications?*`, req => {
      expect(req.query).not.to.have.property('job_id');
      req.reply({ applications: [active, rejected, { ...older, withdrawn: true }], pagination: { totalCount: 3 } });
    });
    cy.intercept('GET', `${gateway}/v1/applications/${active.applicationId}/status`, { statusRecord: { status: 'APPLICATION_STATUS_SHORTLISTED' } });
    cy.intercept('GET', `${gateway}/v1/applications/${rejected.applicationId}/status`, { statusRecord: { status: 'APPLICATION_STATUS_REJECTED' } });
    cy.intercept('GET', `${gateway}/v1/jobs/${jobId}`, { job: job(jobId, 'Frontend intern', inDays(2)) });
    cy.intercept('GET', `${gateway}/v1/jobs/${otherJobId}`, { job: job(otherJobId, 'Data intern', inDays(20)) });
    cy.intercept('GET', `${gateway}/v1/saved-jobs?*`, { savedJobs: [{ jobId, applicantId: userId }, { jobId: otherJobId, applicantId: userId }], pagination: {} });
    signIn('/applicant');
    cy.contains('main h1', 'Welcome back, Pema').should('be.visible');
    // Direct card items only: each card's status rail is a list of five steps.
    cy.get('section[aria-label="Active applications"] > ul > li').should('have.length', 1).first()
      .should('contain', 'Frontend intern').and('contain', 'Druk Tech').and('contain', 'Applied 14 Nov')
      .find('ol[aria-label^="Application progress: Step 3 of 5, Shortlisted"] [aria-current=step]').should('contain', 'Shortlisted');
    cy.get('section[aria-label="Saved jobs closing soon"] > ul > li').should('have.length', 1).and('contain', 'Frontend intern').and('contain', 'Closes in 2 days');
    // Profile and a validated resume exist, so no hint.
    cy.contains('Create your profile').should('not.exist');
    cy.contains('Upload a resume').should('not.exist');
    noUuidOnPage();
  });

  it('asks for a profile first when there is none', () => {
    login('ROLE_APPLICANT');
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { statusCode: 404, body: { code: 5 } });
    signIn('/applicant');
    cy.contains('h2', 'Create your profile').should('be.visible');
    cy.contains('a', /^Create profile$/).should('have.attr', 'href', '/applicant/profile');
    cy.contains('No active applications').should('be.visible');
  });
});

describe('JT-110 admin home', () => {
  it('shows pending verification and open report counts from totalCount', () => {
    login('ROLE_ADMIN');
    cy.intercept('GET', `${gateway}/v1/verification/requests?*`, req => {
      expect(req.query.status).to.eq('REQUEST_STATUS_PENDING');
      req.reply({ requests: [], pagination: { totalCount: 7 } });
    });
    cy.intercept('GET', `${gateway}/v1/moderation/reports?*`, req => {
      expect(req.query.status).to.eq('REPORT_STATUS_OPEN');
      req.reply({ reports: [], pagination: { totalCount: 3 } });
    });
    signIn('/admin');
    cy.contains('a', 'Pending verification').should('contain', '7').and('have.attr', 'href', '/admin/verification');
    cy.contains('a', 'Open reports').should('contain', '3').and('have.attr', 'href', '/admin/moderation');
  });
});
