export function initOAuth2SettingsDisableCheckbox() {
  for (const e of document.querySelectorAll('.disable-setting')) e.addEventListener('change', ({target}) => {
    if (target.checked) {
      document.querySelector(e.getAttribute('data-target')).classList.add('disabled');
    } else {
      document.querySelector(e.getAttribute('data-target')).classList.remove('disabled');
    }
  });
}
