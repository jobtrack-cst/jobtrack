// Drive the actual mobile drawer; desktop links need no extra interaction.
export function openNavigation() {
  cy.get('body').then($body => {
    const trigger = $body.find('button[aria-label="Open navigation"]');
    if (trigger.is(':visible') && trigger.attr('aria-expanded') !== 'true') {
      cy.get('[role=dialog]').should('not.exist');
      cy.wrap(trigger).click().should('have.attr', 'aria-expanded', 'true');
    }
    if (trigger.is(':visible')) cy.get('[role=dialog]').should('be.visible');
  });
}

export function closeNavigation() {
  cy.get('body').then($body => {
    const trigger = $body.find('button[aria-label="Open navigation"]');
    if (trigger.is(':visible')) {
      if (trigger.attr('aria-expanded') === 'true') cy.get('button[aria-label="Close navigation"]').click();
      cy.get('[role=dialog]').should('not.exist');
    }
  });
}
