import {toggleElem} from '../utils/dom.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

export function initUserSettings() {
  registerGlobalInitFunc('initUserSettingsForm', (elForm: HTMLFormElement) => {
    const usernameInput = elForm.querySelector<HTMLInputElement>('input[name=name]')!;
    usernameInput.addEventListener('input', () => {
      const prompt = elForm.querySelector('#name-change-prompt')!;
      const promptRedirect = elForm.querySelector('#name-change-redirect-prompt')!;
      const showPrompt = usernameInput.value.toLowerCase() !== usernameInput.getAttribute('data-name')!.toLowerCase();
      toggleElem(prompt, showPrompt);
      toggleElem(promptRedirect, showPrompt);
    });
  });
}
