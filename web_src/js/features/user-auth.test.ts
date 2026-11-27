import {initUserAuthSubmitLoading} from './user-auth.ts';

afterEach(() => document.body.replaceChildren());

test('initUserAuthSubmitLoading prevents duplicate 2FA submissions', () => {
  document.body.innerHTML = `
    <form class="js-twofa-form">
      <input name="passcode" value="123456" required>
      <button type="submit" class="ui primary button">Verify</button>
    </form>
  `;
  const form = document.querySelector<HTMLFormElement>('.js-twofa-form')!;
  const button = form.querySelector<HTMLButtonElement>('button')!;
  initUserAuthSubmitLoading();

  let preventedByInit = false;
  form.addEventListener('submit', (e) => {
    preventedByInit = e.defaultPrevented;
    e.preventDefault(); // keep the test iframe from navigating
  });

  form.dispatchEvent(new SubmitEvent('submit', {bubbles: true, cancelable: true, submitter: button}));
  expect(preventedByInit).toBe(false);
  expect(button.disabled).toBe(true);
  expect(button.classList.contains('is-loading')).toBe(true);
  expect(button.classList.contains('loading-icon-2px')).toBe(true);

  form.dispatchEvent(new SubmitEvent('submit', {bubbles: true, cancelable: true, submitter: button}));
  expect(preventedByInit).toBe(true);
});
