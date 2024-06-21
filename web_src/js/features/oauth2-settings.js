export function initOAuth2SettingsDisableCheckbox() {
  document.querySelectorAll('.disable-setting').forEach(e => e.addEventListener('change',  ({ target }) => {
    if (target.checked) {
      document.querySelector(e.dataset.target).classList.add('disabled');
    } else {
      document.querySelector(e.dataset.target).classList.remove('disabled');
    }
  }));
}
