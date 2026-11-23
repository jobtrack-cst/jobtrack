// Shared form-field checks for the stubbed specs.

// Finds the input or textarea under a field label (TextInput: label above the input).
// A multiline field also renders a hidden textarea for sizing; the visible one comes first.
export function field(label) {
  return cy.contains('label', new RegExp(`^${Cypress._.escapeRegExp(label)}`)).parent().find('input, textarea').first();
}

// Long text is set with the native value setter and one input event: typing it key by key
// trips React's nested update limit inside Cypress, which does not happen in a real browser (JT-83).
export function fill(label, text) {
  field(label).then($el => {
    const el = $el[0];
    const view = el.ownerDocument.defaultView;
    const proto = el.tagName === 'TEXTAREA' ? view.HTMLTextAreaElement.prototype : view.HTMLInputElement.prototype;
    Object.getOwnPropertyDescriptor(proto, 'value').set.call(el, text);
    el.dispatchEvent(new view.Event('input', { bubbles: true }));
  });
}

// Left edge of where an input's text starts: its box plus its own border and padding.
function textLeft(el) {
  const style = el.ownerDocument.defaultView.getComputedStyle(el);
  return el.getBoundingClientRect().left + parseFloat(style.borderLeftWidth) + parseFloat(style.paddingLeft);
}

// How far the text starts inside the field's own input box (the bordered box drawn by TextInput).
const textInset = el => textLeft(el) - el.closest('.MuiInputBase-root').getBoundingClientRect().left;

function compare(singleLabel, multilineLabel, measure, what) {
  field(singleLabel).should('match', 'input').then($single => {
    field(multilineLabel).should('match', 'textarea').then($multi => {
      expect(Math.abs(measure($single[0]) - measure($multi[0])), `${what} of ${singleLabel} and ${multilineLabel}`).to.be.at.most(0.5);
    });
  });
}

// JT-111: a multiline TextInput must start its text at the same left edge as a
// single-line one (the textarea once kept the input padding on top of the root's).
// For two fields in the same container.
export const expectSameTextEdge = (singleLabel, multilineLabel) => compare(singleLabel, multilineLabel, textLeft, 'text edge');

// The same rule for fields in different containers (for example a field inside a
// padded fieldset): the text must start the same distance inside each input box.
export const expectSameTextInset = (singleLabel, multilineLabel) => compare(singleLabel, multilineLabel, textInset, 'text inset');
