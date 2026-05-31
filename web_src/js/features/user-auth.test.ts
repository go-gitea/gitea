import {initUserAuthSubmitLoading} from './user-auth.ts';

test('initUserAuthSubmitLoading disables submit button after first submit', () => {
  document.body.innerHTML = `
    <form class="js-twofa-form">
      <input name="passcode" value="123456" required>
      <button class="ui primary button">Verify</button>
    </form>
  `;

  const form = document.querySelector<HTMLFormElement>('.js-twofa-form')!;
  const button = form.querySelector<HTMLButtonElement>('button')!;
  initUserAuthSubmitLoading();

  const firstSubmit = new SubmitEvent('submit', {bubbles: true, cancelable: true, submitter: button});
  expect(form.dispatchEvent(firstSubmit)).toBe(true);
  expect(firstSubmit.defaultPrevented).toBe(false);
  expect(button.disabled).toBe(true);
  expect(button.classList.contains('is-loading')).toBe(true);
  expect(button.classList.contains('loading-icon-2px')).toBe(true);

  const secondSubmit = new SubmitEvent('submit', {bubbles: true, cancelable: true, submitter: button});
  expect(form.dispatchEvent(secondSubmit)).toBe(false);
  expect(secondSubmit.defaultPrevented).toBe(true);
});
