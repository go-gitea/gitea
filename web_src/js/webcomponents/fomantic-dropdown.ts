/**
 * Dropdown web component.
 *
 * This component wraps a Fomantic UI dropdown, allowing you to apply the same
 * classes and attributes as you would with a standard Fomantic dropdown.
 *
 * It ensures automatic initialization when connected to the DOM, which is useful
 * for dynamically added elements, eliminating the need for manual initialization.
 */
class FomanticDropdown extends HTMLElement {
  connectedCallback() {
    if (window.jQuery) {
      window.$(this).dropdown();
    }
        // note: if jquery is not defined then this component was part of the initial page load and
        // will be initialized by the fomantic-ui js
  }
}

customElements.define('fomantic-dropdown', FomanticDropdown);
