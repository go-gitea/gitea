import {hideElem, showElem} from '../utils/dom.ts';
import {initCompCropper} from './comp/Cropper.ts';

function initUserSettingsAvatarCropper() {
  const fileInput = document.querySelector<HTMLInputElement>('#new-avatar');
  const container = document.querySelector<HTMLElement>('.user.settings.profile .cropper-panel');
  const imageSource = container.querySelector<HTMLImageElement>('.cropper-source');
  initCompCropper({container, fileInput, imageSource});
}

export function initUserSettings() {
  if (!document.querySelector('.user.settings.profile')) return;

  initUserSettingsAvatarCropper();

  const usernameInput = document.querySelector('#username');
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
