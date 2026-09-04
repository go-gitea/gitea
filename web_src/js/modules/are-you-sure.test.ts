import {applyAreYouSure, initGlobalFormDirtyLeaveConfirm, shouldTriggerAreYouSure} from './are-you-sure.ts';

function createForm(fieldsHtml: string, wrapperClass = ''): HTMLFormElement {
  document.body.insertAdjacentHTML('beforeend', `<div class="${wrapperClass}"><form>${fieldsHtml}</form></div>`);
  return document.body.lastElementChild!.querySelector('form')!;
}

const field = (form: HTMLFormElement, name: string) => form.elements.namedItem(name) as HTMLInputElement;

function set(field: HTMLInputElement, props: Partial<HTMLInputElement>, eventType = 'change') {
  Object.assign(field, props);
  field.dispatchEvent(new Event(eventType, {bubbles: true}));
}

const isBeforeUnloadPrevented = () => !window.dispatchEvent(new Event('beforeunload', {cancelable: true}));

afterEach(() => document.body.replaceChildren());

test('tracks changes via input, change and keyup, fires onDirtyChange on flips, submit and reset clear the state', () => {
  const form = createForm(`
    <input name="title" value="a">
    <input type="checkbox" name="flag">
    <select name="pick"><option value="a" selected>a</option><option value="b">b</option></select>
  `);
  const onDirtyChange = vi.fn();
  applyAreYouSure(form, onDirtyChange);
  set(field(form, 'title'), {value: 'b'}, 'input');
  set(field(form, 'title'), {value: 'c'}, 'input');
  set(field(form, 'title'), {value: 'a'}, 'keyup');
  set(field(form, 'flag'), {checked: true});
  set(field(form, 'flag'), {checked: false});
  set(field(form, 'pick'), {value: 'b'});
  expect(onDirtyChange.mock.calls).toEqual([[true], [false], [true], [false], [true]]);
  expect(shouldTriggerAreYouSure()).toBe(true);
  form.addEventListener('submit', (e) => e.preventDefault());
  form.dispatchEvent(new SubmitEvent('submit', {cancelable: true}));
  expect(shouldTriggerAreYouSure()).toBe(false);
  set(field(form, 'title'), {value: 'b'});
  form.reset();
  expect(shouldTriggerAreYouSure()).toBe(false);
});

test('ignores unnamed, ays-ignore, submit and button inputs and fields added after initialization', () => {
  const form = createForm(`
    <input value="a">
    <input name="dummy" class="ays-ignore" value="a">
    <div class="ays-ignore"><input name="summary" value="a"></div>
    <input type="submit" name="send" value="a">
    <input type="button" name="act" value="a">
  `);
  applyAreYouSure(form);
  form.insertAdjacentHTML('beforeend', '<input name="later" value="a">');
  for (const input of form.querySelectorAll('input')) set(input, {value: 'b'});
  expect(shouldTriggerAreYouSure()).toBe(false);
});

test('applying again resets the baseline, replaces the callback and re-evaluates ays-ignore, removed fields stop counting', () => {
  const form = createForm(`
    <input name="title" value="a">
    <input name="extra" value="a">
    <div class="wrapper"><input name="summary" value="a"></div>
  `);
  const oldOnDirtyChange = vi.fn();
  const onDirtyChange = vi.fn();
  applyAreYouSure(form, oldOnDirtyChange);
  set(field(form, 'title'), {value: 'b'});
  form.querySelector('.wrapper')!.classList.add('ays-ignore');
  applyAreYouSure(form, onDirtyChange);
  set(field(form, 'summary'), {value: 'b'});
  set(field(form, 'extra'), {value: 'b'});
  field(form, 'extra').remove();
  set(field(form, 'title'), {value: 'b'});
  set(field(form, 'title'), {value: 'a'});
  expect(oldOnDirtyChange.mock.calls).toEqual([[true]]);
  expect(onDirtyChange.mock.calls).toEqual([[true], [false], [true]]);
});

test('initGlobalFormDirtyLeaveConfirm skips the sign-in page and ignore-dirty forms, prevents beforeunload only while a visible non-ignored form is dirty', () => {
  const signin = createForm('<input name="user_name" value="a">', 'page-content user signin');
  initGlobalFormDirtyLeaveConfirm();
  set(field(signin, 'user_name'), {value: 'b'});
  expect(shouldTriggerAreYouSure()).toBe(false);
  signin.parentElement!.remove();
  const form = createForm('<input name="title" value="a">');
  const ignored = createForm('<input name="q" value="a">');
  ignored.classList.add('ignore-dirty');
  initGlobalFormDirtyLeaveConfirm();
  set(field(ignored, 'q'), {value: 'b'});
  ignored.classList.remove('ignore-dirty');
  expect(isBeforeUnloadPrevented()).toBe(false);
  set(field(form, 'title'), {value: 'b'});
  expect(isBeforeUnloadPrevented()).toBe(true);
  form.parentElement!.classList.add('tw-hidden');
  expect(isBeforeUnloadPrevented()).toBe(false);
  form.parentElement!.classList.remove('tw-hidden');
  form.classList.add('ignore-dirty');
  expect(isBeforeUnloadPrevented()).toBe(false);
});
