export function initOAuth2SettingsDisableCheckbox() {
  for (const e of document.querySelectorAll('.disable-setting')) {
    e.addEventListener('change', (e: Event & {target: HTMLInputElement}) => {
      document.querySelector(e.target.getAttribute('data-target')).classList.toggle('disabled', e.target.checked);
    });
  }
}
