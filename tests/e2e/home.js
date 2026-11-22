// JT-110 home page headings: the applicant is greeted by first name; employer and admin see "Home".
export const homeHeading = (role, fullName = '') =>
  role === 'applicant' ? (fullName.trim() ? `Welcome back, ${fullName.trim().split(/\s+/)[0]}` : 'Welcome back') : 'Home';

// The home pages load lists of their own after login. Wait for them, so stubs
// and aliases registered afterwards only see the page under test.
export function settleHome() {
  cy.get('.workspace-content h1').should('be.visible');
  cy.get('.workspace-content [role=status][aria-label^="Loading"]').should('not.exist');
}

// Employers reach applications through Applicants (JT-110 menus).
export const openApplications = (role = 'ROLE_APPLICANT') =>
  cy.contains('nav a', role === 'ROLE_EMPLOYER' ? /^Applicants$/ : /^Applications$/).click();

// JT-110 main navigation per role (redesign doc Section 3). Messages joins with the inbox in JT-113.
export const navigationLinks = {
  applicant: ['Home', 'Find jobs', 'Saved', 'Applications', 'Profile', 'Job alerts'],
  employer: ['Home', 'Jobs', 'Applicants', 'Company'],
  admin: ['Home', 'Verification', 'Moderation', 'Analytics', 'Audit log'],
};
