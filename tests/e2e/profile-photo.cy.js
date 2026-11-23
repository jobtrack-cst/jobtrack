import { openAccountMenu } from './account-menu';
// JT-103 applicant profile photo. Gateway responses are stubbed, so these tests
// check the UI and the requests it sends (see docs/testing/jt-103-test-cases.md).
const gateway = 'http://localhost:8000';
const userId = '11111111-1111-4111-8111-111111111111';
// A real 1x1 PNG, so the browser can display it.
const pngBase64 = 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII=';
const png = () => Cypress.Buffer.from(pngBase64, 'base64');
const jpegHeader = [0xff, 0xd8, 0xff];
const pngHeader = [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a];
const file = (header, size) => {
  const bytes = Cypress.Buffer.alloc(size);
  Cypress.Buffer.from(header).copy(bytes);
  return bytes;
};
const profile = { applicantId: userId, summary: 'Synthetic summary', education: [], skills: [] };

// photo is what GET /v1/applicant/profile/photo answers right after login (the app bar asks for it).
function login(role = 'ROLE_APPLICANT', photo = { statusCode: 404, body: { code: 5, message: 'private' } }) {
  const user = { userId, email: 'pema@example.test', fullName: 'Pema Dorji', role };
  cy.intercept('POST', `${gateway}/v1/auth/login`, { accessToken: 'jt103-token', expiresAt: new Date(Date.now() + 3600000).toISOString(), user });
  cy.intercept('GET', `${gateway}/v1/auth/me`, { user });
  // Every request must go through the shared client with the JWT.
  cy.intercept(`${gateway}/v1/**`, req => {
    if (!req.url.endsWith('/auth/login')) expect(req.headers.authorization).to.eq('Bearer jt103-token');
  });
  cy.intercept('GET', `${gateway}/v1/applicant/profile/photo`, photo).as('photo');
  cy.visit('/login');
  cy.get('input[name=email]').type('pema@example.test');
  cy.get('input[name=password]').type('password123', { log: false });
  cy.get('button[type=submit]').click();
  cy.location('pathname').should('eq', role === 'ROLE_APPLICANT' ? '/applicant' : '/employer');
}
function openProfile(body = { profile }) {
  cy.intercept('GET', `${gateway}/v1/applicant/profile`, body).as('profile');
  cy.contains('a', 'Profile').click();
  cy.wait('@profile');
}
function choose(contents, fileName = 'photo.png', mimeType = 'image/png') {
  cy.get('input[type=file][name=photo]').selectFile({ contents, fileName, mimeType }, { force: true });
}
const appBarPhoto = () => cy.get('header img[alt="Pema Dorji profile photo"]');
const pagePhoto = () => cy.get('main img[alt="Pema Dorji profile photo"]');

describe('JT-103 applicant profile photo', () => {
  it('shows initials in the app bar and on the profile page when there is no photo', () => {
    login();
    cy.wait('@photo');
    cy.get('header').contains('PD').should('be.visible');
    openProfile();
    cy.contains('h2', 'Profile photo').should('be.visible');
    cy.get('main').contains('PD').should('be.visible');
    cy.contains('label', 'Upload photo').should('be.visible');
    cy.contains('JPEG or PNG, up to 1,000,000 bytes').should('be.visible');
  });

  it('shows an existing photo in the app bar and on the profile page', () => {
    login('ROLE_APPLICANT', { contentType: 'image/png', content: pngBase64 });
    appBarPhoto().should('have.attr', 'src', `data:image/png;base64,${pngBase64}`);
    openProfile({ profile: { ...profile, hasPhoto: true } });
    pagePhoto().should('have.attr', 'src', `data:image/png;base64,${pngBase64}`);
    cy.contains('label', 'Replace photo').should('be.visible');
    // The app bar and the page share one request.
    cy.get('@photo.all').should('have.length', 1);
  });

  it('uploads a PNG as base64 and shows it everywhere without loading it again', () => {
    login();
    openProfile();
    cy.intercept('PUT', `${gateway}/v1/applicant/profile/photo`, req => {
      expect(req.body).to.deep.eq({ contentType: 'image/png', content: pngBase64 });
      req.reply({ profile: { ...profile, hasPhoto: true } });
    }).as('upload');
    choose(png());
    cy.wait('@upload');
    cy.contains('Photo added.').should('be.visible');
    pagePhoto().should('have.attr', 'src', `data:image/png;base64,${pngBase64}`);
    appBarPhoto().should('have.attr', 'src', `data:image/png;base64,${pngBase64}`);
    cy.contains('label', 'Replace photo').should('be.visible');
    cy.get('@photo.all').should('have.length', 1);
  });

  it('replaces an existing photo with a JPEG', () => {
    login('ROLE_APPLICANT', { contentType: 'image/png', content: pngBase64 });
    openProfile({ profile: { ...profile, hasPhoto: true } });
    const jpeg = file(jpegHeader, 3);
    cy.intercept('PUT', `${gateway}/v1/applicant/profile/photo`, req => {
      expect(req.body).to.deep.eq({ contentType: 'image/jpeg', content: jpeg.toString('base64') });
      req.reply({ profile: { ...profile, hasPhoto: true } });
    }).as('upload');
    choose(jpeg, 'photo.jpg', 'image/jpeg');
    cy.wait('@upload');
    cy.contains('Photo replaced.').should('be.visible');
  });

  it('sends a file of exactly 1,000,000 bytes', () => {
    login();
    openProfile();
    cy.intercept('PUT', `${gateway}/v1/applicant/profile/photo`, req => {
      expect(Cypress.Buffer.from(req.body.content, 'base64').length).to.eq(1_000_000);
      req.reply({ profile: { ...profile, hasPhoto: true } });
    }).as('upload');
    choose(file(pngHeader, 1_000_000));
    cy.wait('@upload');
    cy.contains('Photo added.').should('be.visible');
  });

  // Contents are factories, so each test gets its own buffer (as in the JT-83 resume tests).
  for (const [name, contents, fileName, mimeType, message] of [
    ['an empty file', () => Cypress.Buffer.alloc(0), 'photo.png', 'image/png', 'The photo must be 1 to 1,000,000 bytes.'],
    ['a file of 1,000,001 bytes', () => file(pngHeader, 1_000_001), 'photo.png', 'image/png', 'The photo must be 1 to 1,000,000 bytes.'],
    ['a GIF', () => file([0x47, 0x49, 0x46, 0x38], 10), 'photo.gif', 'image/gif', 'The photo must be a JPEG or PNG file.'],
    ['a PNG name with JPEG bytes', () => file(jpegHeader, 10), 'photo.png', 'image/png', 'The file is not a real JPEG or PNG image.'],
    ['a PDF renamed to JPEG', () => Cypress.Buffer.from('%PDF-1.7 synthetic'), 'photo.jpg', 'image/jpeg', 'The file is not a real JPEG or PNG image.'],
  ]) {
    it(`rejects ${name} before uploading`, () => {
      login();
      openProfile();
      cy.intercept('PUT', `${gateway}/v1/applicant/profile/photo`, { profile }).as('upload');
      choose(contents(), fileName, mimeType);
      cy.contains(message).should('be.visible');
      cy.get('@upload.all').should('have.length', 0);
    });
  }

  for (const [status, code, message] of [
    [400, 3, 'The photo was not saved. Use a JPEG or PNG image of 1 to 1,000,000 bytes.'],
    [404, 5, 'Create your profile before you add a photo.'],
    [403, 7, 'Only applicants can have a profile photo.'],
    [413, 3, 'The photo file is too large. Use an image of at most 1,000,000 bytes.'],
  ]) {
    it(`shows a fixed message when the gateway answers ${status}`, () => {
      login();
      openProfile();
      cy.intercept('PUT', `${gateway}/v1/applicant/profile/photo`, { statusCode: status, body: { code, message: 'server detail' } }).as('upload');
      choose(png());
      cy.wait('@upload');
      cy.contains(message).should('be.visible');
      cy.contains('server detail').should('not.exist');
      cy.get('main').contains('PD').should('be.visible');
    });
  }

  it('offers the upload only after the profile exists', () => {
    login();
    openProfile({ statusCode: 404, body: { code: 5, message: 'private' } });
    cy.contains('Save your profile to add a photo.').should('be.visible');
    cy.get('input[type=file][name=photo]').should('not.exist');
  });

  it('shows no photo and asks for none for employers', () => {
    login('ROLE_EMPLOYER');
    openAccountMenu();
    cy.contains('button', 'Log out').should('be.visible');
    cy.get('header .MuiAvatar-root').should('not.exist');
    cy.get('@photo.all').should('have.length', 0);
  });
});
