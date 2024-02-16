import {hideElem, showElem} from '../utils/dom.js';

export function initUserSettings() {
  if (document.querySelectorAll('.user.settings.profile').length === 0) return;

  const usernameInput = document.getElementById('username');
  if (!usernameInput) return;
  usernameInput.addEventListener('input', function () {
    const prompt = document.getElementById('name-change-prompt');
    const promptRedirect = document.getElementById('name-change-redirect-prompt');
    if (this.value.toLowerCase() !== this.getAttribute('data-name').toLowerCase()) {
      showElem(prompt);
      showElem(promptRedirect);
    } else {
      hideElem(prompt);
      hideElem(promptRedirect);
    }
  });
}
