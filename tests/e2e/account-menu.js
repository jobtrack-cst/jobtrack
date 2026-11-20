export function openAccountMenu() {
  // History updates before React commits login and role-guard redirects.
  cy.get('main .workspace-content').should('be.visible');
  cy.get('#account-menu-button').should('have.attr', 'aria-expanded', 'false').click();
  cy.get('#account-menu-button').should('have.attr', 'aria-expanded', 'true');
  cy.get('#account-menu [role=menu]').should('be.visible');
  cy.get('#account-menu .MuiPaper-root').should('have.css', 'transform', 'none');
}
