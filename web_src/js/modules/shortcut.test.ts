import {initGlobalShortcut} from './shortcut.ts';

beforeEach(() => {
  document.body.innerHTML = `
    <div id="keyboard-shortcuts-modal" class="hidden">
      <div class="modal-backdrop"></div>
      <div class="content">
        <button class="close-button">Close</button>
      </div>
    </div>
    <div class="secondary-nav">
      <a href="/owner/repo/issues">Issues</a>
      <a href="/owner/repo/pulls">Pull Requests</a>
      <a href="/owner/repo/actions">Actions</a>
    </div>
    <input type="text" id="test-input">
  `;
});

afterEach(() => {
  vi.unstubAllGlobals();
});

test('toggles keyboard shortcuts modal on pressing ?', () => {
  initGlobalShortcut();

  const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal')!;
  expect(helpModal.classList.contains('hidden')).toBe(true);

  const event = new KeyboardEvent('keydown', {key: '?', bubbles: true});
  document.dispatchEvent(event);

  expect(helpModal.classList.contains('hidden')).toBe(false);

  document.dispatchEvent(event);
  expect(helpModal.classList.contains('hidden')).toBe(true);
});

test('does not toggle keyboard shortcuts modal when pressing ? in input', () => {
  initGlobalShortcut();

  const helpModal = document.querySelector<HTMLElement>('#keyboard-shortcuts-modal')!;
  const input = document.querySelector<HTMLInputElement>('#test-input')!;

  expect(helpModal.classList.contains('hidden')).toBe(true);

  const event = new KeyboardEvent('keydown', {key: '?', bubbles: true});
  const spy = vi.spyOn(event, 'preventDefault');

  input.dispatchEvent(event);

  expect(helpModal.classList.contains('hidden')).toBe(true);
  expect(spy).not.toHaveBeenCalled();
});

test('handles navigation sequences (g + key) and restarts sequence on double g', () => {
  initGlobalShortcut();

  const assignMock = vi.fn();
  vi.stubGlobal('location', {...window.location, assign: assignMock});
  vi.stubGlobal('config', {appSubUrl: '/sub'});

  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'd', bubbles: true}));

  expect(assignMock).toHaveBeenCalledWith('/sub/');
  assignMock.mockClear();

  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'd', bubbles: true}));

  expect(assignMock).toHaveBeenCalledWith('/sub/');
  assignMock.mockClear();

  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'a', bubbles: true}));

  expect(assignMock).toHaveBeenCalledWith('/owner/repo/actions');
});

test('does not navigate to a repo unit that is not available', () => {
  initGlobalShortcut();

  const assignMock = vi.fn();
  vi.stubGlobal('location', {...window.location, assign: assignMock});

  document.querySelector('.secondary-nav a[href$="/actions"]')?.remove();

  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'a', bubbles: true}));

  expect(assignMock).not.toHaveBeenCalled();
});

test('falls back to global issues when the repo has no issues tab', () => {
  initGlobalShortcut();

  const assignMock = vi.fn();
  vi.stubGlobal('location', {...window.location, assign: assignMock});
  vi.stubGlobal('config', {appSubUrl: '/sub'});

  document.querySelector('.secondary-nav a[href$="/issues"]')?.remove();

  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'g', bubbles: true}));
  document.dispatchEvent(new KeyboardEvent('keydown', {key: 'i', bubbles: true}));

  expect(assignMock).toHaveBeenCalledWith('/sub/issues');
});
