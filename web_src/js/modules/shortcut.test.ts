import '../../fomantic/build/fomantic.js';
import {userEvent} from 'vitest/browser';
import {initGlobalShortcut, makeFullKeyFromEvent} from './shortcut.ts';
import {initGlobalSelectorObserver} from './observer.ts';
import {initGiteaFomantic} from './fomantic.ts';

function press(key: string, options: KeyboardEventInit = {}, target: HTMLElement = document.body) {
  const event = new KeyboardEvent('keydown', {key, bubbles: true, cancelable: true, ...options});
  target.dispatchEvent(event);
  return event;
}

beforeAll(() => {
  document.body.innerHTML = `
    <button data-shortcut-keys="g i"> <span> </span><span>Issues</span><span>42</span></button>
    <div class="global-shortcut-wrapper">
      <input placeholder="Search code">
      <kbd data-global-init="onGlobalShortcut" data-shortcut-keys="s">S</kbd>
    </div>
    <textarea></textarea>
    <select><option>Value</option></select>
    <div contenteditable="true"><span>Editable</span></div>
  `;
  initGiteaFomantic();
  initGlobalShortcut();
  initGlobalSelectorObserver(null);
});

beforeEach(() => press('Escape'));
afterEach(() => vi.restoreAllMocks());
afterAll(() => document.body.replaceChildren());

test('clicks matching sequences and clears completed or expired presses', () => {
  const click = vi.spyOn(document.querySelector('button')!, 'click');
  const now = vi.spyOn(Date, 'now').mockReturnValue(1000);
  press('x');
  press('g');
  expect(press('i').defaultPrevented).toBe(false);
  press('Escape');
  press('G');
  expect(click).not.toHaveBeenCalled();
  expect(press('I').defaultPrevented).toBe(true);
  expect(click).toHaveBeenCalledTimes(1);
  press('i');
  press('g');
  now.mockReturnValue(2001);
  expect(press('i').defaultPrevented).toBe(false);
  expect(click).toHaveBeenCalledTimes(1);
  press('g');
  press('x');
  press('i');
  expect(click).toHaveBeenCalledTimes(1);
});

test('ignores editing, modifiers, composition and repeated keydown events', () => {
  const click = vi.spyOn(document.querySelector('button')!, 'click');
  for (const target of document.querySelectorAll<HTMLElement>('input, textarea, select, [contenteditable] span')) {
    press('g');
    press('x', {}, target);
    press('i');
  }
  for (const options of [{ctrlKey: true}, {metaKey: true}, {altKey: true}, {isComposing: true}]) {
    press('g');
    press('i', options);
    press('i');
  }
  press('g');
  press('Escape');
  press('i');
  press('Escape');
  press('g');
  press('i', {repeat: true});
  expect(click).not.toHaveBeenCalled();
  press('i');
  expect(click).toHaveBeenCalledTimes(1);
});

test('keeps input focus, hint visibility and Escape behavior', () => {
  const input = document.querySelector('input')!;
  const kbd = document.querySelector('kbd')!;
  expect(press('S').defaultPrevented).toBe(true);
  expect(document.activeElement).toBe(input);
  expect(kbd.classList.contains('tw-hidden')).toBe(true);
  input.value = 'search';
  press('Escape', {}, input);
  expect(input.value).toBe('');
  expect(document.activeElement).not.toBe(input);
  expect(kbd.classList.contains('tw-hidden')).toBe(false);
});

test('normalizes keys and sorts modifiers alphabetically', () => {
  for (const [options, expected] of [
    [{key: 'V'}, 'V'],
    [{key: 'v'}, 'V'],
    [{key: 'V', ctrlKey: true, shiftKey: true}, 'Control+Shift+V'],
    [{key: 'a', ctrlKey: true, altKey: true, metaKey: true, shiftKey: true}, 'Alt+Control+Meta+Shift+A'],
    [{key: ' ', shiftKey: true}, 'Shift+Space'],
    [{key: '+', altKey: true}, 'Alt+Plus'],
    [{key: 'ArrowUp', metaKey: true}, 'Meta+ArrowUp'],
    [{key: 'PageDown'}, 'PageDown'],
    [{key: 'F12'}, 'F12'],
    [{key: 'Escape'}, 'Escape'],
    ...['Control', 'Alt', 'Meta', 'Shift', ''].map((key) => [{key}, '']),
  ] as [KeyboardEventInit, string][]) {
    expect(makeFullKeyFromEvent(new KeyboardEvent('keydown', options))).toBe(expected);
  }
});

test('shows current shortcuts with ? and closes with Escape or the close button', async () => {
  const input = document.querySelector('input')!;
  const click = vi.spyOn(document.querySelector('button')!, 'click');
  press('?', {shiftKey: true}, input);
  expect(document.querySelector('#global-shortcut-help')).toBeNull();
  press('?', {ctrlKey: true});
  expect(document.querySelector('#global-shortcut-help')).toBeNull();
  press('g');
  expect(press('?', {shiftKey: true}).defaultPrevented).toBe(true);
  const modal = document.querySelector<HTMLElement>('#global-shortcut-help')!;
  expect(modal.classList.contains('active')).toBe(true);
  expect(modal.querySelector('.shortcut-list tr:nth-child(2) td:last-child')!.textContent).toBe('Issues');
  expect(modal.querySelector('.shortcut-list')!.textContent).toContain('Search code');
  expect(modal.querySelectorAll('.shortcut-list tr')).toHaveLength(3);
  press('g');
  press('i');
  expect(click).not.toHaveBeenCalled();
  await userEvent.keyboard('{Escape}');
  expect(modal.classList.contains('active')).toBe(false);

  input.setAttribute('placeholder', '<b>Updated search</b>');
  press('?');
  expect(document.querySelectorAll('.ui.modal')).toHaveLength(1);
  expect(modal.querySelector('.shortcut-list')!.textContent).toContain('<b>Updated search</b>');
  expect(modal.querySelector('.shortcut-list b')).toBeNull();
  expect(modal.querySelectorAll('.shortcut-list tr')).toHaveLength(3);
});
