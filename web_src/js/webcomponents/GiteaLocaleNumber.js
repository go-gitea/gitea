// Convert a number to a locale string by data-number attribute.
// Or add a tooltip by data-number-in-tooltip attribute. JSON: {message: "count: %s", number: 123}
window.customElements.define('gitea-locale-number', class extends HTMLElement {
  connectedCallback() {
    const number = this.getAttribute('data-number');
    if (number) {
      this.attachShadow({mode: 'open'});
      this.shadowRoot.textContent = new Intl.NumberFormat().format(Number(number));
    }
    let numberInTooltip = this.getAttribute('data-number-in-tooltip');
    if (numberInTooltip) {
      numberInTooltip = JSON.parse(numberInTooltip);
      let msg = numberInTooltip.message;
      msg = msg.replace(/%[ds]/, new Intl.NumberFormat().format(Number(numberInTooltip.number)));
      this.setAttribute('data-tooltip-content', msg);
    }
  }
});
