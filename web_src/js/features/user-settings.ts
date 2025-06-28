import {hideElem, showElem} from '../utils/dom.ts';

export function initUserSettings() {
  if (!document.querySelector('.user.settings.profile')) return;

  const usernameInput = document.querySelector<HTMLInputElement>('#username');
  if (!usernameInput) return;
  usernameInput.addEventListener('input', function () {
    const prompt = document.querySelector('#name-change-prompt');
    const promptRedirect = document.querySelector('#name-change-redirect-prompt');
    if (this.value.toLowerCase() !== this.getAttribute('data-name').toLowerCase()) {
      showElem(prompt);
      showElem(promptRedirect);
    } else {
      hideElem(prompt);
      hideElem(promptRedirect);
    }
  });
}
