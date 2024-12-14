export function initOAuth2SettingsDisableCheckbox() {
  for (const el of document.querySelectorAll('.disable-setting')) {
    el.addEventListener('change', (e: Event & {target: HTMLInputElement}) => {
      document.querySelector(e.target.getAttribute('data-target')).classList.toggle('disabled', e.target.checked);
    });
  }
}
