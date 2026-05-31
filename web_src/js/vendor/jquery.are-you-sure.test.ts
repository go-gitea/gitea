import {applyAreYouSure, initAreYouSure, shouldTriggerAreYouSure} from './jquery.are-you-sure.ts';

type AreYouSureWindow = Window & typeof globalThis & {
  aysHasPrompted?: boolean;
  aysUnloadSet?: boolean;
};

function changeInputValue(input: HTMLInputElement, value: string) {
  input.setAttribute('value', value);
  window.jQuery(input).val(value).trigger('input');
}

test('dirty form state is restored when another dirty form blocks unload', async () => {
  delete (window as AreYouSureWindow).aysHasPrompted;
  delete (window as AreYouSureWindow).aysUnloadSet;
  document.body.innerHTML = `
    <form id="profile"><input name="name" value="old"></form>
    <form id="avatar"><input name="avatar" value="old"></form>
  `;

  initAreYouSure(window.jQuery);
  applyAreYouSure('form:not(.ignore-dirty)', {fieldSelector: 'input'});

  const profileForm = document.querySelector<HTMLFormElement>('#profile')!;
  const avatarForm = document.querySelector<HTMLFormElement>('#avatar')!;

  changeInputValue(profileForm.querySelector<HTMLInputElement>('input')!, 'new name');
  changeInputValue(avatarForm.querySelector<HTMLInputElement>('input')!, 'new avatar');
  window.jQuery(profileForm).trigger('checkform.areYouSure');
  window.jQuery(avatarForm).trigger('checkform.areYouSure');

  expect(profileForm.classList.contains('dirty')).toBe(true);
  expect(avatarForm.classList.contains('dirty')).toBe(true);

  profileForm.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
  expect(profileForm.classList.contains('dirty')).toBe(false);
  expect(shouldTriggerAreYouSure()).toBe(true);

  await new Promise((resolve) => window.setTimeout(resolve, 0));
  expect(profileForm.classList.contains('dirty')).toBe(true);

  avatarForm.dispatchEvent(new Event('submit', {bubbles: true, cancelable: true}));
  expect(shouldTriggerAreYouSure()).toBe(true);
});
