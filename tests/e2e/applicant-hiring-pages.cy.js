// JT-83 pages. Gateway responses are stubbed, so these tests check the UI and
// the requests it sends, not live services (see docs/testing/jt-83-test-cases.md).
const gateway = 'http://localhost:8000';
// Keep the applicant app-bar photo request independent of a running gateway.
beforeEach(() => cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, { statusCode: 404, body: {} }));
const userId = '11111111-1111-4111-8111-111111111111';
const year = new Date().getFullYear();

function login(role = 'ROLE_APPLICANT') {
  const user = { userId, email: 'pema@example.test', fullName: 'Pema', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt83-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt83-token');
  });
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', role === 'ROLE_APPLICANT' ? '/applicant' : '/employer');
}
function field(label) {
  return cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
}
// Long text is set with the native value setter and one input event, as in the
// JT-78 offer test: typing it key by key trips React's nested update limit
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

describe('JT-83 applicant profile', () => {
  const profile = {
    applicantId: userId, summary: 'Synthetic summary',
    education: [{ institution: 'Synthetic College', qualification: 'BSc', startYear: 2020, endYear: 2024 }],
    skills: ['Go', 'React'],
  };
  // A saved profile opens read-only (JT-109); Edit profile switches to the form.
  const editProfile = () => cy.contains('button', 'Edit profile').click();

  it('creates a profile when the applicant has none', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { statusCode: 404, body: { code: 5, message: 'private' } }).as('load');
    login();
    cy.contains('a', 'Profile').click();
    cy.wait('@load');
    cy.contains('You do not have a profile yet').should('be.visible');
    field('Summary').type('Synthetic summary');
    cy.contains('button', 'Add education').click();
    field('Institution').type('Synthetic College');
    field('Qualification').type('BSc');
    field('Start year').type('2020');
    field('End year').type('2024');
    field('Skills').type('Go{enter}React{enter}');
    cy.intercept('POST', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body).to.deep.eq({ summary: profile.summary, education: profile.education, skills: profile.skills });
      req.reply({ profile });
    }).as('create');
    cy.contains('button', 'Create profile').click();
    cy.wait('@create');
    cy.contains('Profile created.').should('be.visible');
    cy.contains('Synthetic College, BSc, 2020 to 2024').should('be.visible');
    cy.get('textarea[name=summary]').should('not.exist');
    cy.contains('button', 'Edit profile').should('be.visible');
  });

  it('updates an existing profile with PUT, sending an ongoing entry as end year 0', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile }).as('load');
    login();
    cy.contains('a', 'Profile').click();
    cy.wait('@load');
    editProfile();
    field('Summary').should('have.value', 'Synthetic summary');
    field('End year').clear();
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body.education).to.deep.eq([{ ...profile.education[0], endYear: 0 }]);
      expect(req.body.skills).to.deep.eq(['Go', 'React']);
      req.reply({ profile: { ...profile, education: req.body.education } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
    cy.contains('Profile updated.').should('be.visible');
    cy.contains('Synthetic College, BSc, 2020 to ongoing').should('be.visible');
    cy.contains('button', 'Edit profile').should('be.visible');
  });

  it('removes an education entry and clears the list', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    editProfile();
    cy.contains('button', 'Remove entry 1').click();
    cy.contains('No education entries.').should('be.visible');
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, req => {
      expect(req.body.education).to.deep.eq([]);
      req.reply({ profile: { ...profile, education: [] } });
    }).as('update');
    cy.contains('button', 'Save profile').click();
    cy.wait('@update');
  });

  for (const [name, change, message] of [
    ['start year below 1950', () => { field('Start year').clear().type('1949'); }, 'start year must be from 1950'],
    ['start year after next year', () => { field('Start year').clear().type(String(year + 2)); }, 'start year must be from 1950'],
    ['end year before start year', () => { field('End year').clear().type('2019'); }, 'end year must be empty'],
    ['end year more than 10 years ahead', () => { field('End year').clear().type(String(year + 11)); }, 'end year must be empty'],
    ['empty institution', () => { field('Institution').clear(); }, 'institution needs 1 to 150'],
    ['institution over 150 characters', () => { fill('Institution', 'x'.repeat(151)); }, 'institution needs 1 to 150'],
    ['duplicate skill ignoring case', () => { field('Skills').type('go{enter}'); }, 'listed twice'],
  ]) {
    it(`blocks saving with ${name}`, () => {
      cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
      login();
      cy.contains('a', 'Profile').click();
      editProfile();
      field('Summary').should('have.value', 'Synthetic summary');
      change();
      cy.get('[role=alert]').should('contain', message);
      cy.contains('button', 'Save profile').should('be.disabled');
    });
  }

  it('accepts boundary values: 150 character institution, start year next year, end year 10 years ahead', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    editProfile();
    fill('Institution', 'x'.repeat(150));
    field('Institution').should('have.value', 'x'.repeat(150));
    field('Start year').clear().type(String(year + 1));
    field('End year').clear().type(String(year + 10));
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a fixed message for a rejected save and never the server text', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, { statusCode: 400, body: { code: 3, message: 'private-input' } });
    login();
    cy.contains('a', 'Profile').click();
    editProfile();
    field('Summary').type(' changed');
    cy.contains('button', 'Save profile').click();
    cy.get('[role=alert]').should('contain', 'The profile was not saved').and('not.contain', 'private-input');
    cy.contains('button', 'Save profile').should('be.enabled');
  });

  it('shows a load failure and retries', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { statusCode: 503, body: {} });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('The request failed').should('be.visible');
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    cy.contains('button', 'Try again').click();
    cy.contains('Synthetic summary').should('be.visible');
  });

  it('shows the saved profile read-only with an Edit button (JT-109)', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('Synthetic summary').should('be.visible');
    cy.contains('Synthetic College, BSc, 2020 to 2024').should('be.visible');
    cy.get('main .MuiChip-root').should('have.length', 2).and('contain', 'Go').and('contain', 'React');
    cy.get('main input, main textarea').not('[type=file]').should('not.exist');
    cy.contains('button', 'Save profile').should('not.exist');
    cy.contains('button', 'Edit profile').should('be.visible');
  });

  it('shows empty sections of a saved profile read-only (JT-109)', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile: { applicantId: userId } });
    login();
    cy.contains('a', 'Profile').click();
    cy.contains('No summary.').should('be.visible');
    cy.contains('No education entries.').should('be.visible');
    cy.contains('No skills.').should('be.visible');
  });

  it('cancels an edit without saving and restores the saved values (JT-109)', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    cy.intercept('PUT', `${gateway}/v1/applicant/profile`, { profile }).as('update');
    login();
    cy.contains('a', 'Profile').click();
    editProfile();
    field('Summary').clear().type('Unsaved change');
    cy.contains('button', 'Remove entry 1').click();
    cy.contains('button', 'Cancel').click();
    cy.contains('Synthetic summary').should('be.visible');
    cy.contains('Synthetic College, BSc, 2020 to 2024').should('be.visible');
    cy.contains('Unsaved change').should('not.exist');
    editProfile();
    field('Summary').should('have.value', 'Synthetic summary');
    cy.get('@update.all').should('have.length', 0);
  });

  it('disables Save until something changes (JT-109)', () => {
    cy.intercept('GET', `${gateway}/v1/applicant/profile`, { profile });
    login();
    cy.contains('a', 'Profile').click();
    editProfile();
    cy.contains('button', 'Save profile').should('be.disabled');
    cy.contains('No changes to save.').should('be.visible');
    field('Summary').type('!');
    cy.contains('button', 'Save profile').should('be.enabled');
    cy.contains('No changes to save.').should('not.exist');
    field('Summary').type('{backspace}');
    cy.contains('button', 'Save profile').should('be.disabled');
  });

  it('does not show the profile link to employers', () => {
    login('ROLE_EMPLOYER');
    cy.contains('a', 'Profile').should('not.exist');
  });
});

describe('JT-83 resume upload', () => {
  const resumeId = '33333333-3333-4333-8333-333333333333';
  const pending = { resumeId, fileName: 'cv.pdf', contentType: 'application/pdf', sizeBytes: '9', validationStatus: 'RESUME_VALIDATION_STATUS_PENDING', timestamps: { createdAt: '2026-11-15T10:00:00Z' } };
  const pdf = (size = 9) => Cypress.Buffer.concat([Cypress.Buffer.from('%PDF-1.7\n'), Cypress.Buffer.alloc(Math.max(0, size - 9), 0x20)]).subarray(0, size);
  function openResumes(resumes = []) {
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes, pagination: {} }).as('list');
    login();
    cy.contains('a', 'Resumes').click();
    cy.wait('@list');
  }
  function choose(contents, fileName = 'cv.pdf', mimeType = 'application/pdf') {
    cy.get('input[type=file][name=resume]').selectFile({ contents, fileName, mimeType }, { force: true });
  }

  it('uploads a PDF as base64 and lists it as pending', () => {
    openResumes();
    cy.contains('No resumes on this page.').should('be.visible');
    cy.intercept('POST', `${gateway}/v1/resumes`, req => {
      expect(req.body).to.deep.eq({ fileName: 'cv.pdf', contentType: 'application/pdf', content: pdf().toString('base64') });
      req.reply({ resume: pending });
    }).as('upload');
    cy.intercept('GET', `${gateway}/v1/resumes?*`, req => {
      expect(req.query['pagination.page_token']).to.eq('');
      req.reply({ resumes: [pending], pagination: {} });
    }).as('relist');
    choose(pdf());
    cy.wait('@upload');
    cy.wait('@relist');
    cy.contains('cv.pdf uploaded').should('be.visible');
    cy.contains('Pending validation').should('be.visible');
  });

  it('accepts a file name of exactly 255 characters', () => {
    const name = `${'n'.repeat(251)}.pdf`;
    openResumes();
    cy.intercept('POST', `${gateway}/v1/resumes`, req => {
      expect(req.body.fileName).to.eq(name);
      req.reply({ resume: { ...pending, fileName: name } });
    }).as('upload');
    choose(pdf(), name);
    cy.wait('@upload');
    cy.contains('uploaded').should('be.visible');
  });

  it('accepts a file of exactly 2,000,000 bytes', () => {
    openResumes();
    cy.intercept('POST', `${gateway}/v1/resumes`, req => {
      expect(Cypress.Buffer.from(req.body.content, 'base64').length).to.eq(2000000);
      req.reply({ resume: { ...pending, sizeBytes: '2000000' } });
    }).as('upload');
    choose(pdf(2000000));
    cy.wait('@upload');
    cy.contains('uploaded').should('be.visible');
  });

  for (const [name, contents, fileName, mimeType, message] of [
    ['a file over 2,000,000 bytes', () => pdf(2000001), 'big.pdf', 'application/pdf', 'larger than 2 MB'],
    ['a file that is not a PDF', () => Cypress.Buffer.from('hello'), 'cv.docx', 'application/vnd.openxmlformats-officedocument.wordprocessingml.document', 'Only PDF files'],
    ['an empty file', () => Cypress.Buffer.alloc(0), 'empty.pdf', 'application/pdf', 'The file is empty'],
    ['a PDF type without the %PDF- signature', () => Cypress.Buffer.from('not really a pdf'), 'fake.pdf', 'application/pdf', 'not a valid PDF'],
    ['a file name over 255 characters', () => pdf(), `${'n'.repeat(252)}.pdf`, 'application/pdf', 'file name must have 1 to 255'],
    ['a file name with a slash', () => pdf(), 'cv/1.pdf', 'application/pdf', 'file name must have 1 to 255'],
    ['a file name with a backslash', () => pdf(), 'cv\\1.pdf', 'application/pdf', 'file name must have 1 to 255'],
    ['a file name with a NUL character', () => pdf(), 'cv\u00001.pdf', 'application/pdf', 'file name must have 1 to 255'],
  ]) {
    it(`rejects ${name} before uploading`, () => {
      let posted = false;
      cy.intercept('POST', `${gateway}/v1/resumes`, () => { posted = true; });
      openResumes();
      choose(contents(), fileName, mimeType);
      cy.get('[role=alert]').should('contain', message);
      cy.then(() => expect(posted, 'upload request sent').to.eq(false));
      cy.contains('label', 'Upload PDF resume').should('not.have.attr', 'aria-disabled');
    });
  }

  for (const [statusCode, body, message] of [
    [400, { code: 3, message: 'private-input' }, 'The resume was not accepted'],
    [413, 'request body too large private-input', 'The file is too large'],
    [403, { code: 7, message: 'private-input' }, 'Only applicants can upload'],
    [503, {}, 'The request failed'],
  ]) {
    it(`shows a clear message when the gateway answers ${statusCode}`, () => {
      openResumes();
      cy.intercept('POST', `${gateway}/v1/resumes`, { statusCode, body });
      choose(pdf());
      cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
    });
  }

  it('shows each validation status, the rejection reason and pages through resumes', () => {
    const validated = { ...pending, resumeId: '33333333-3333-4333-8333-333333333334', fileName: 'good.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_VALIDATED' };
    const rejected = { ...pending, resumeId: '33333333-3333-4333-8333-333333333335', fileName: 'bad.pdf', validationStatus: 'RESUME_VALIDATION_STATUS_REJECTED', rejectionReason: 'File signature is not a PDF' };
    cy.intercept('GET', `${gateway}/v1/resumes?*`, req => {
      expect(req.query['pagination.page_size']).to.eq('20');
      req.reply(req.query['pagination.page_token'] === '20' ? { resumes: [rejected], pagination: {} } : { resumes: [pending, validated], pagination: { nextPageToken: '20' } });
    });
    login();
    cy.contains('a', 'Resumes').click();
    cy.contains('Pending validation').should('be.visible');
    cy.contains('Validated').should('be.visible');
    cy.contains('button', 'Next page').click();
    cy.contains('bad.pdf').should('be.visible');
    cy.contains('Reason: File signature is not a PDF').should('be.visible');
    cy.contains('button', 'First page').click();
    cy.contains('good.pdf').should('be.visible');
  });

  it('refreshes to show a resume that was validated after upload', () => {
    openResumes([pending]);
    cy.intercept('GET', `${gateway}/v1/resumes?*`, { resumes: [{ ...pending, validationStatus: 'RESUME_VALIDATION_STATUS_VALIDATED' }], pagination: {} }).as('again');
    cy.contains('button', 'Refresh resumes').click();
    cy.wait('@again');
    cy.contains('Validated').should('be.visible');
  });
});

describe('JT-83 application status, review and interviews', () => {
  const applicationId = '44444444-4444-4444-8444-444444444444';
  const application = { applicationId, jobId: '22222222-2222-4222-8222-222222222222', resumeId: '33333333-3333-4333-8333-333333333333', applicantId: userId, employerId: '66666666-6666-4666-8666-666666666666', withdrawn: false };
  const statusUrl = `${gateway}/v1/applications/${applicationId}/status`;
  const historyUrl = `${gateway}/v1/applications/${applicationId}/status-history?*`;
  const reviewUrl = `${gateway}/v1/applications/${applicationId}/review`;
  const reviewsUrl = `${gateway}/v1/applications/${applicationId}/reviews`;
  const interviewsUrl = `${gateway}/v1/applications/${applicationId}/interviews`;
  const pad = n => String(n).padStart(2, '0');
  // datetime-local value for a time daysAhead from now, in the browser's zone.
  const localInput = (daysAhead, hour = 10) => {
    const d = new Date(Date.now() + daysAhead * 86400000);
    d.setHours(hour, 0, 0, 0);
    return { value: `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(hour)}:00`, iso: d.toISOString() };
  };
  function openCard(role, button) {
    cy.intercept('GET', `${gateway}/v1/applications?*`, { applications: [application], pagination: {} }).as('applications');
    login(role);
    cy.contains('a', 'Applications').click();
    cy.wait('@applications');
    if (button) cy.contains('a', new RegExp(`^${button}$`)).click();
  }

  describe('status and history', () => {
    it('shows the current status and paged history from the application card', () => {
      cy.intercept('GET', statusUrl, { statusRecord: { applicationId, status: 'APPLICATION_STATUS_SHORTLISTED', timestamps: { updatedAt: '2026-11-15T10:00:00Z' } } });
      cy.intercept('GET', historyUrl, req => {
        expect(req.query['pagination.page_size']).to.eq('20');
        req.reply(req.query['pagination.page_token'] === '20'
          ? { entries: [{ previousStatus: 'APPLICATION_STATUS_UNDER_REVIEW', newStatus: 'APPLICATION_STATUS_SHORTLISTED', occurredAt: '2026-11-15T10:00:00Z' }], pagination: {} }
          : { entries: [
            { previousStatus: 'APPLICATION_STATUS_UNSPECIFIED', newStatus: 'APPLICATION_STATUS_SUBMITTED', occurredAt: '2026-11-14T10:00:00Z' },
            { previousStatus: 'APPLICATION_STATUS_SUBMITTED', newStatus: 'APPLICATION_STATUS_UNDER_REVIEW', occurredAt: '2026-11-15T09:00:00Z' },
          ], pagination: { nextPageToken: '20' } });
      });
      openCard('ROLE_APPLICANT', 'Status');
      cy.location('pathname').should('eq', `/applicant/applications/${applicationId}/status`);
      cy.contains('Current status:').parent().should('contain', 'Shortlisted');
      cy.get('table[aria-label="Status history"] tbody tr').should('have.length', 2).first().should('contain', 'None').and('contain', 'Submitted');
      cy.contains('button', 'Next page').click();
      cy.get('table[aria-label="Status history"] tbody tr').should('have.length', 1).and('contain', 'Under review');
    });

    for (const [statusCode, code, message] of [[404, 5, 'has no status yet'], [403, 7, 'Only the applicant and the employer']]) {
      it(`explains a ${statusCode} status response`, () => {
        cy.intercept('GET', statusUrl, { statusCode, body: { code, message: 'private-input' } });
        cy.intercept('GET', historyUrl, { statusCode, body: { code, message: 'private-input' } });
        openCard('ROLE_EMPLOYER', 'Status');
        cy.get('[role=alert]').should('have.length', 1).and('contain', message).and('not.contain', 'private-input');
      });
    }

    it('rejects an application ID in the address that is not a UUID without calling the gateway', () => {
      let called = false;
      cy.intercept('GET', `${gateway}/v1/applications/not-a-uuid/**`, () => { called = true; });
      login();
      cy.window().then(win => { win.history.pushState({}, '', '/applicant/applications/not-a-uuid/status'); win.dispatchEvent(new PopStateEvent('popstate')); });
      cy.contains('The application ID in the address is not valid.').should('be.visible');
      cy.then(() => expect(called).to.eq(false));
    });
  });

  describe('employer review', () => {
    it('completes a review after confirmation and then shows it read-only', () => {
      cy.intercept('GET', reviewUrl, { statusCode: 404, body: { code: 5 } });
      openCard('ROLE_EMPLOYER', 'Review');
      cy.contains('button', 'Complete review').should('be.disabled');
      cy.contains('label', 'Score').parent().find('[role=combobox]').click();
      cy.contains('[role=option]', /^4$/).click();
      cy.contains('label', 'Shortlist').click();
      cy.intercept('POST', reviewsUrl, req => {
        expect(req.body).to.deep.eq({ score: 4, decision: 'REVIEW_DECISION_SHORTLISTED' });
        req.reply({ review: { reviewId: '77777777-7777-4777-8777-777777777777', applicationId, score: 4, decision: 'REVIEW_DECISION_SHORTLISTED', timestamps: { createdAt: '2026-11-15T10:00:00Z' } } });
      }).as('complete');
      cy.contains('button', 'Complete review').click();
      cy.contains('Shortlist this application?').should('be.visible');
      cy.contains('button', 'Cancel').click();
      cy.get('@complete.all').should('have.length', 0);
      cy.contains('button', 'Complete review').click();
      cy.contains('button', 'Confirm review').click();
      cy.wait('@complete');
      cy.contains('Review completed').should('be.visible');
      cy.contains('Score: 4 of 5').should('be.visible');
      cy.contains('button', 'Complete review').should('not.exist');
    });

    it('shows an existing review without the form', () => {
      cy.intercept('GET', reviewUrl, { review: { reviewId: 'r', applicationId, score: 2, decision: 'REVIEW_DECISION_REJECTED', timestamps: { createdAt: '2026-11-15T10:00:00Z' } } });
      openCard('ROLE_EMPLOYER', 'Review');
      cy.contains('Decision: Rejected').should('be.visible');
      cy.contains('button', 'Complete review').should('not.exist');
    });

    for (const [statusCode, code, message] of [[409, 6, 'already been reviewed'], [404, 5, 'not available for review yet'], [403, 7, 'Only the employer who posted'], [400, 3, 'Choose a score from 1 to 5']]) {
      it(`explains a ${statusCode} answer to Complete review`, () => {
        cy.intercept('GET', reviewUrl, { statusCode: 404, body: { code: 5 } });
        cy.intercept('POST', reviewsUrl, { statusCode, body: { code, message: 'private-input' } });
        openCard('ROLE_EMPLOYER', 'Review');
        cy.contains('label', 'Score').parent().find('[role=combobox]').click();
        cy.contains('[role=option]', /^1$/).click();
        cy.contains('label', 'Reject').click();
        cy.contains('button', 'Complete review').click();
        cy.contains('button', 'Confirm review').click();
        cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
        cy.contains('button', 'Complete review').should('be.enabled');
      });
    }

    it('gives applicants no review link or route', () => {
      openCard('ROLE_APPLICANT');
      cy.contains('a', 'Status').should('be.visible');
      cy.contains('a', /^Review$/).should('not.exist');
      cy.window().then(win => { win.history.pushState({}, '', `/applicant/applications/${applicationId}/review`); win.dispatchEvent(new PopStateEvent('popstate')); });
      cy.location('pathname').should('eq', '/applicant');
    });
  });

  describe('interviews', () => {
    const interview = { interviewId: '88888888-8888-4888-8888-888888888888', applicationId, scheduledAt: '2026-11-20T04:00:00Z', details: 'Room 2\nBring your ID' };

    it('lists interviews for the applicant without a schedule form', () => {
      cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [interview], pagination: {} });
      openCard('ROLE_APPLICANT', 'Interviews');
      cy.contains('Room 2').should('be.visible');
      cy.contains('Schedule an interview').should('not.exist');
    });

    it('schedules an interview in UTC from the employer local time and refreshes the list', () => {
      const when = localInput(3);
      cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [], pagination: {} });
      openCard('ROLE_EMPLOYER', 'Interviews');
      cy.contains('No interviews scheduled').should('be.visible');
      field('Date and time').type(when.value);
      field('Details').type('  Room 2  ');
      cy.intercept('POST', interviewsUrl, req => {
        expect(req.body).to.deep.eq({ scheduledAt: when.iso, details: 'Room 2' });
        req.reply({ interview: { ...interview, scheduledAt: when.iso, details: 'Room 2' } });
      }).as('schedule');
      cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [{ ...interview, scheduledAt: when.iso, details: 'Room 2' }], pagination: {} }).as('relist');
      cy.contains('button', 'Schedule interview').click();
      cy.wait(['@schedule', '@relist']);
      cy.contains('Interview scheduled for').should('be.visible');
      field('Date and time').should('have.value', '');
    });

    for (const [name, days, message] of [['in the past', -1, 'in the future'], ['more than 365 days ahead', 367, 'within 365 days']]) {
      it(`blocks a time ${name}`, () => {
        cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [], pagination: {} });
        openCard('ROLE_EMPLOYER', 'Interviews');
        field('Date and time').type(localInput(days).value);
        cy.get('[role=alert]').should('contain', message);
        cy.contains('button', 'Schedule interview').should('be.disabled');
      });
    }

    it('allows 1000 characters of details and blocks 1001', () => {
      cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [], pagination: {} });
      openCard('ROLE_EMPLOYER', 'Interviews');
      field('Date and time').type(localInput(2).value);
      fill('Details', 'x'.repeat(1000));
      cy.contains('button', 'Schedule interview').should('be.enabled');
      fill('Details', 'x'.repeat(1001));
      cy.get('[role=alert]').should('contain', 'at most 1000 characters');
      cy.contains('button', 'Schedule interview').should('be.disabled');
    });

    for (const [statusCode, code, message] of [[409, 6, 'already scheduled at this time'], [400, 3, 'The interview was not scheduled'], [403, 7, 'Only the employer who posted'], [404, 5, 'not available yet']]) {
      it(`explains a ${statusCode} answer to Schedule interview`, () => {
        cy.intercept('GET', `${interviewsUrl}?*`, { interviews: [], pagination: {} });
        cy.intercept('POST', interviewsUrl, { statusCode, body: { code, message: 'private-input' } });
        openCard('ROLE_EMPLOYER', 'Interviews');
        field('Date and time').type(localInput(2).value);
        cy.contains('button', 'Schedule interview').click();
        cy.get('[role=alert]').should('contain', message).and('not.contain', 'private-input');
        cy.contains('button', 'Schedule interview').should('be.enabled');
      });
    }
  });
});
