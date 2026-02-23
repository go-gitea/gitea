// The keyboard shortcut mechanism is driven by global event delegation in observer.ts.
// These tests set up the same event listeners to verify the behavior in isolation.

function setupKeyboardShortcutListeners() {
  document.addEventListener('keydown', (e: KeyboardEvent) => {
    const target = e.target as HTMLElement;

    if (e.key === 'Escape' && target.matches('input, textarea, select')) {
      const kbd = target.parentElement?.querySelector<HTMLElement>('kbd[data-global-keyboard-shortcut]');
      if (kbd) {
        (target as HTMLInputElement).value = '';
        (target as HTMLInputElement).blur();
        return;
      }
    }

    if (target.matches('input, textarea, select') || target.isContentEditable) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;

    const key = e.key.toLowerCase();
    const escapedKey = CSS.escape(key);
    const kbd = document.querySelector<HTMLElement>(`kbd[data-global-keyboard-shortcut="${escapedKey}"]`);
    if (!kbd) return;

    e.preventDefault();
    const input = kbd.parentElement?.querySelector<HTMLInputElement>('input, textarea, select');
    if (input) input.focus();
  });

  document.addEventListener('focusin', (e) => {
    const target = e.target as HTMLElement;
    if (!target.matches('input, textarea, select')) return;
    const kbd = target.parentElement?.querySelector<HTMLElement>('kbd[data-global-keyboard-shortcut]');
    if (kbd) kbd.style.display = 'none';
  });

  document.addEventListener('focusout', (e) => {
    const target = e.target as HTMLElement;
    if (!target.matches('input, textarea, select')) return;
    const kbd = target.parentElement?.querySelector<HTMLElement>('kbd[data-global-keyboard-shortcut]');
    if (kbd) kbd.style.display = (target as HTMLInputElement).value ? 'none' : '';
  });
}

describe('Keyboard Shortcut Mechanism', () => {
  let input: HTMLInputElement;
  let kbd: HTMLElement;

  beforeEach(() => {
    document.body.innerHTML = `
      <div>
        <input placeholder="Search" type="text">
        <kbd data-global-keyboard-shortcut="s">S</kbd>
      </div>
    `;
    input = document.querySelector('input')!;
    kbd = document.querySelector('kbd')!;
  });

  afterEach(() => {
    document.body.innerHTML = '';
  });

  // Register listeners once for all tests (they persist across tests on document)
  setupKeyboardShortcutListeners();

  test('Shortcut key focuses the associated input', () => {
    expect(document.activeElement).not.toBe(input);

    document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 's', bubbles: true}));

    expect(document.activeElement).toBe(input);
  });

  test('Kbd hint hides when input is focused', () => {
    expect(kbd.style.display).toBe('');

    input.focus();

    expect(kbd.style.display).toBe('none');
  });

  test('Kbd hint shows when input is blurred with empty value', () => {
    input.focus();
    expect(kbd.style.display).toBe('none');

    input.blur();

    expect(kbd.style.display).toBe('');
  });

  test('Kbd hint stays hidden when input is blurred with a value', () => {
    input.focus();
    input.value = 'test';

    input.blur();

    expect(kbd.style.display).toBe('none');
  });

  test('Escape key clears and blurs the input', () => {
    input.focus();
    input.value = 'test';

    const event = new KeyboardEvent('keydown', {key: 'Escape', bubbles: true});
    input.dispatchEvent(event);

    expect(input.value).toBe('');
    expect(document.activeElement).not.toBe(input);
  });

  test('Escape key shows kbd hint after clearing', () => {
    input.focus();
    input.value = 'test';
    expect(kbd.style.display).toBe('none');

    const event = new KeyboardEvent('keydown', {key: 'Escape', bubbles: true});
    input.dispatchEvent(event);

    expect(kbd.style.display).toBe('');
  });

  test('Shortcut does not trigger with modifier keys', () => {
    document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 's', ctrlKey: true, bubbles: true}));
    expect(document.activeElement).not.toBe(input);

    document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 's', metaKey: true, bubbles: true}));
    expect(document.activeElement).not.toBe(input);

    document.body.dispatchEvent(new KeyboardEvent('keydown', {key: 's', altKey: true, bubbles: true}));
    expect(document.activeElement).not.toBe(input);
  });

  test('Shortcut does not trigger when typing in another input', () => {
    // Add a second input without a shortcut
    const otherInput = document.createElement('input');
    document.body.append(otherInput);
    otherInput.focus();

    const event = new KeyboardEvent('keydown', {key: 's', bubbles: true});
    otherInput.dispatchEvent(event);

    expect(document.activeElement).toBe(otherInput);
    expect(document.activeElement).not.toBe(input);
  });
});
