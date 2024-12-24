import type {DOMEvent} from '../utils/dom.ts';

export function initOAuth2SettingsDisableCheckbox() {
  for (const el of document.querySelectorAll<HTMLInputElement>('.disable-setting')) {
    el.addEventListener('change', (e: DOMEvent<Event, HTMLInputElement>) => {
      document.querySelector(e.target.getAttribute('data-target')).classList.toggle('disabled', e.target.checked);
    });
  }
}
