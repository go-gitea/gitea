export function initOAuth2SettingsDisableCheckbox() {
  for (const e of document.querySelectorAll('.disable-setting')) e.addEventListener('change', ({target}) => {
    document.querySelector(e.getAttribute('data-target')).classList.toggle('disabled', target.checked);
  });
}
