import {initCompWebHookEditor} from './WebHookEditor.ts';
import {applyAreYouSure, shouldTriggerAreYouSure} from '../../modules/are-you-sure.ts';

test('select and deselect all events toggle every event checkbox and mark the form dirty', () => {
  document.body.innerHTML = `
    <form class="new webhook">
      <div class="events fields">
        <button type="button" data-events-checked="true"></button>
        <button type="button" data-events-checked="false"></button>
        <input type="checkbox" name="create" checked>
        <input type="checkbox" name="push">
      </div>
    </form>
  `;
  applyAreYouSure(document.querySelector('form')!);
  initCompWebHookEditor();
  const eventsChecked = () => Array.from(document.querySelectorAll<HTMLInputElement>('input'), (input) => input.checked);
  document.querySelector<HTMLButtonElement>('[data-events-checked="true"]')!.click();
  expect(eventsChecked()).toEqual([true, true]);
  expect(shouldTriggerAreYouSure()).toBe(true);
  document.querySelector<HTMLButtonElement>('[data-events-checked="false"]')!.click();
  expect(eventsChecked()).toEqual([false, false]);
});
