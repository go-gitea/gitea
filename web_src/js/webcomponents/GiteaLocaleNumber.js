// Convert a number to a locale string by data-number attribute.
// Or add a tooltip by data-number-in-tooltip attribute. JSON: {message: "count: %s", number: 123}
window.customElements.define('gitea-locale-number', class extends HTMLElement {
  connectedCallback() {
    // ideally, the number locale formatting and plural processing should be done by backend with translation strings.
    // if we have complete backend locale support (eg: Golang "x/text" package), we can drop this component.
    const number = this.getAttribute('data-number');
    if (number) {
      this.attachShadow({mode: 'open'});
      this.shadowRoot.textContent = new Intl.NumberFormat().format(Number(number));
    }
    const numberInTooltip = this.getAttribute('data-number-in-tooltip');
    if (numberInTooltip) {
      // TODO: only 2 usages of this, we can replace it with Golang's "x/text/number" package in the future
      const {message, number} = JSON.parse(numberInTooltip);
      const tooltipContent = message.replace(/%[ds]/, new Intl.NumberFormat().format(Number(number)));
      this.setAttribute('data-tooltip-content', tooltipContent);
    }
  }
});
