export function openAccountMenu() {
  cy.get('#account-menu-button').then($button => {
    if ($button.attr('aria-expanded') !== 'true') cy.wrap($button).click();
  });
  cy.get('#account-menu [role=menu]').should('be.visible');
  cy.get('#account-menu .MuiPaper-root').should('have.css', 'transform', 'none');
}
