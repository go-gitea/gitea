import {initCodespaceCreateForm, initCodespaceLiveState} from './codespace.ts';
import {hideFomanticModal, showFomanticModal} from '../modules/fomantic/modal.ts';
import {captureNavigations} from '../utils/testhelper.ts';

vi.mock('../modules/fomantic/modal.ts', () => ({hideFomanticModal: vi.fn(), showFomanticModal: vi.fn()}));
vi.mock('../modules/fomantic/base.ts', () => ({
  fomanticQuery: (select: HTMLSelectElement) => ({
    dropdown: (_action: string, value: string) => {
      select.value = value;
    },
  }),
}));

beforeEach(() => {
  vi.clearAllMocks();
  document.documentElement.lang = 'en-US';
});

test('codespace create environment selection updates its explanation', () => {
  document.body.innerHTML = `
    <form data-codespace-create-form>
      <div class="dropdown">
        <input name="environment_tag" value="standard">
        <div data-codespace-environment-text>standard</div>
        <div class="codespace-create-environment-option" data-value="standard"></div>
        <div class="codespace-create-environment-option" data-value="large"></div>
      </div>
      <div data-codespace-environment-detail="standard"></div>
      <div data-codespace-environment-detail="large" hidden></div>
    </form>`;

  initCodespaceCreateForm();
  document.querySelector<HTMLElement>('[data-value="large"]')!.click();

  expect(document.querySelector<HTMLInputElement>('input[name="environment_tag"]')!.value).toBe('large');
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-text]')!.textContent).toBe('large');
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="standard"]')!.hidden).toBe(true);
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="large"]')!.hidden).toBe(false);

  document.querySelector<HTMLInputElement>('input[name="environment_tag"]')!.value = '';
  const submitEvent = new SubmitEvent('submit', {cancelable: true});
  document.querySelector<HTMLFormElement>('form')!.dispatchEvent(submitEvent);
  expect(submitEvent.defaultPrevented).toBe(true);
  expect(document.querySelector('.dropdown')!.classList.contains('error')).toBe(true);
  expect(document.querySelector('.dropdown')!.getAttribute('aria-invalid')).toBe('true');
  document.body.replaceChildren();
});

test('codespace create configuration preview preserves its source and environment', () => {
  const navigations = captureNavigations();
  document.body.innerHTML = `
    <form data-codespace-create-form>
      <input name="environment_tag" value="standard">
      <select data-codespace-dev-container data-preview-url="/owner/repo/codespaces/new"
        data-ref-type="branch" data-ref-name="main">
        <option value=".devcontainer/devcontainer.json">Default</option>
        <option value=".devcontainer/node/devcontainer.json">Node</option>
      </select>
    </form>`;

  initCodespaceCreateForm();
  const select = document.querySelector<HTMLSelectElement>('[data-codespace-dev-container]')!;
  select.value = '.devcontainer/node/devcontainer.json';
  select.dispatchEvent(new Event('change'));

  const previewURL = new URL(navigations.at(-1)!.url);
  expect(previewURL.pathname).toBe('/owner/repo/codespaces/new');
  expect(Object.fromEntries(previewURL.searchParams)).toEqual({
    ref_type: 'branch',
    ref_name: 'main',
    dev_container: '.devcontainer/node/devcontainer.json',
    environment_tag: 'standard',
  });
  document.body.replaceChildren();
});

test('initCodespaceLiveState opens the gateway recovery modal', () => {
  document.body.innerHTML = '<form id="codespace-open-modal" data-codespace-auto-open="true"></form>';

  initCodespaceLiveState();

  expect(showFomanticModal).toHaveBeenCalledWith(document.querySelector('#codespace-open-modal'));
  document.body.replaceChildren();
});

test('initCodespaceLiveState closes the source modal after opening a new tab', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = '<form id="codespace-open-modal" target="_blank" class="is-loading"></form>';
    const form = document.querySelector<HTMLFormElement>('#codespace-open-modal')!;

    initCodespaceLiveState();
    form.dispatchEvent(new SubmitEvent('submit'));
    await vi.runAllTimersAsync();

    expect(hideFomanticModal).toHaveBeenCalledWith(form);
    expect(form.classList.contains('is-loading')).toBe(false);
  } finally {
    vi.useRealTimers();
    document.body.replaceChildren();
  }
});

test('initCodespaceLiveState refreshes the state fragment', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = '<div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="10">old</div>';
    const fetchMock = vi.fn().mockResolvedValue(new Response(
      '<div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="0">new</div>',
      {status: 200},
    ));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(10);
    await vi.waitFor(() => expect(document.querySelector('#codespace-live-state')!.textContent).toContain('new'));

    expect(fetchMock).toHaveBeenCalledWith('/-/codespaces/uuid/state', expect.objectContaining({method: 'GET'}));
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace settings button fills and opens the shared auto-stop modal', () => {
  document.body.innerHTML = `
    <button class="codespace-settings-button"
      data-settings-url="/-/codespaces/uuid/auto-stop" data-return-to="/-/codespaces/uuid"
      data-auto-stop-mode="custom" data-auto-stop-timeout-value="2" data-auto-stop-timeout-unit="hours"
      data-auto-stop-configurable="true" data-auto-stop-out-of-range="false"
      data-auto-stop-effective="Stop after two hours" data-auto-stop-default-description="Site default" data-auto-stop-range="One minute to one day"></button>
    <form id="codespace-settings-modal">
      <input name="return_to">
      <p data-auto-stop-effective></p>
      <p data-auto-stop-default-description></p>
      <p data-auto-stop-range></p>
      <div data-auto-stop-unavailable hidden></div>
      <div data-auto-stop-out-of-range hidden></div>
      <fieldset data-auto-stop-fields>
        <input type="radio" name="mode" value="default">
        <input type="radio" name="mode" value="custom">
        <input type="radio" name="mode" value="never">
        <div data-auto-stop-custom-fields hidden>
          <input name="timeout_value">
          <select class="ui dropdown" name="timeout_unit"><option value="hours">hours</option></select>
        </div>
      </fieldset>
      <button type="submit">save</button>
    </form>
  `;

  initCodespaceLiveState();
  document.querySelector<HTMLButtonElement>('.codespace-settings-button')!.click();

  const form = document.querySelector<HTMLFormElement>('#codespace-settings-modal')!;
  expect(form.action).toMatch(/\/-\/codespaces\/uuid\/auto-stop$/);
  expect(form.querySelector<HTMLInputElement>('input[name="return_to"]')!.value).toBe('/-/codespaces/uuid');
  expect(form.querySelector<HTMLInputElement>('input[name="mode"][value="custom"]')!.checked).toBe(true);
  expect(form.querySelector<HTMLInputElement>('input[name="timeout_value"]')!.value).toBe('2');
  expect(form.querySelector<HTMLElement>('[data-auto-stop-custom-fields]')!.hidden).toBe(false);
  expect(form.querySelector<HTMLElement>('[data-auto-stop-effective]')!.textContent).toBe('Stop after two hours');
  expect(showFomanticModal).toHaveBeenCalledWith(form);
  document.body.replaceChildren();
});

function codespaceLogHTML(offset = '0', lineCount = '0', empty = 'true') {
  return `
    <section class="codespace-log-panel">
      <span class="tw-hidden" data-codespace-log-loading>Loading…</span>
      <button data-codespace-log-toggle-timestamps><span class="tw-invisible" data-codespace-log-timestamp-check></span></button>
      <button data-codespace-log-fullscreen data-enter-label="Enter" data-exit-label="Exit"><span>Enter</span></button>
      <div id="codespace-log-view" data-log-url="/-/codespaces/uuid/logs" data-log-next-offset="${offset}"
        data-log-refresh-after-ms="10" data-log-line-count="${lineCount}" data-log-empty="${empty}"
        data-log-error-message="Log unavailable">
        ${empty === 'true' ? '<div data-log-empty-message>Empty</div>' : ''}
      </div>
    </section>
  `;
}

test('initCodespaceLiveState immediately appends structured log lines', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    const fetchMock = vi.fn().mockResolvedValue(Response.json({
      next_offset: 12,
      eof: true,
      operation_active: true,
      lines: [
        {timestamp: 1785037200, message: 'first'},
        {timestamp: 1785037201, message: 'second'},
      ],
    }, {status: 200}));
    vi.stubGlobal('fetch', fetchMock);
    const logView = document.querySelector<HTMLElement>('#codespace-log-view')!;
    Object.defineProperties(logView, {
      clientHeight: {value: 100},
      scrollHeight: {value: 200},
      scrollTop: {value: 100, writable: true},
    });

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    await vi.waitFor(() => expect(logView.querySelectorAll('.codespace-log-line-message')).toHaveLength(2));

    expect(fetchMock).toHaveBeenCalledWith('/-/codespaces/uuid/logs?offset=0', expect.objectContaining({method: 'GET'}));
    expect(Array.from(logView.querySelectorAll('.codespace-log-line-number'), (el) => el.textContent)).toEqual(['1', '2']);
    expect(Array.from(logView.querySelectorAll('.codespace-log-line-message'), (el) => el.textContent)).toEqual(['first', 'second']);
    expect(logView.querySelectorAll('.codespace-log-line-timestamp')).toHaveLength(2);
    expect(logView.getAttribute('data-log-next-offset')).toBe('12');
    expect(logView.getAttribute('data-log-empty')).toBe('false');
    expect(logView.scrollTop).toBe(200);

    document.querySelector<HTMLButtonElement>('[data-codespace-log-toggle-timestamps]')!.click();
    expect(logView.classList.contains('show-timestamps')).toBe(true);
    expect(document.querySelector<HTMLElement>('[data-codespace-log-timestamp-check]')!.classList.contains('tw-invisible')).toBe(false);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log refresh preserves a reader position away from the bottom', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML('12', '2', 'false');
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({
      next_offset: 18,
      eof: true,
      operation_active: true,
      lines: [{timestamp: 1785037202, message: 'third'}],
    }, {status: 200})));
    const logView = document.querySelector<HTMLElement>('#codespace-log-view')!;
    Object.defineProperties(logView, {
      clientHeight: {value: 100},
      scrollHeight: {value: 300},
      scrollTop: {value: 80, writable: true},
    });

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    await vi.waitFor(() => expect(logView.querySelector('.codespace-log-line-message')).not.toBeNull());

    expect(logView.querySelector('.codespace-log-line-message')!.textContent).toBe('third');
    expect(logView.scrollTop).toBe(80);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log renders Actions groups, severities, commands, links, and ANSI messages', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({
      next_offset: 32,
      eof: true,
      operation_active: false,
      lines: [
        {timestamp: 1785037200, message: '##[group]Create #1'},
        {timestamp: 1785037201, message: '\u001b[32mready\u001b[0m'},
        {timestamp: 1785037202, message: '##[warning]check https://example.com/log'},
        {timestamp: 1785037203, message: '##[command]make build'},
        {timestamp: 1785037204, message: '##[error]failed'},
        {timestamp: 1785037205, message: '##[endgroup]'},
      ],
    }, {status: 200})));

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);

    const group = document.querySelector<HTMLDetailsElement>('.codespace-log-group')!;
    expect(group.querySelector('summary')!.textContent).toBe('Create #1');
    expect(group.open).toBe(true);
    expect(group.getAttribute('data-log-error')).toBe('true');
    expect(group.querySelector('.codespace-log-line-message')!.textContent).toBe('ready');
    expect(group.querySelector('.codespace-log-line-warning .log-msg-label')!.textContent).toBe('Warning:');
    expect(group.querySelector<HTMLAnchorElement>('.codespace-log-line-warning a')!.href).toBe('https://example.com/log');
    expect(group.querySelector('.codespace-log-line-message.log-cmd-command')!.textContent).toBe('make build');
    expect(group.querySelector('.codespace-log-line-error .log-msg-label')!.textContent).toBe('Error:');
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log reloads once after an offset conflict', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML('12', '2', 'false');
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, {status: 409}))
      .mockResolvedValueOnce(Response.json({next_offset: 4, eof: true, operation_active: false, lines: [{timestamp: 1785037200, message: 'reloaded'}]}));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(10);

    expect(fetchMock).toHaveBeenNthCalledWith(1, '/-/codespaces/uuid/logs?offset=12', expect.anything());
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/-/codespaces/uuid/logs?offset=0', expect.anything());
    expect(document.querySelector('.codespace-log-line-message')!.textContent).toBe('reloaded');
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log catches up pages without the polling delay and preserves groups', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(Response.json({
        next_offset: 12,
        eof: false,
        operation_active: true,
        lines: [
          {timestamp: 1785037200, message: '##[group]Build'},
          {timestamp: 1785037201, message: 'first page'},
        ],
      }))
      .mockResolvedValueOnce(Response.json({
        next_offset: 24,
        eof: true,
        operation_active: true,
        lines: [
          {timestamp: 1785037202, message: 'second page'},
          {timestamp: 1785037203, message: '##[endgroup]'},
        ],
      }));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(16);

    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(fetchMock).toHaveBeenNthCalledWith(1, '/-/codespaces/uuid/logs?offset=0', expect.anything());
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/-/codespaces/uuid/logs?offset=12', expect.anything());
    expect(Array.from(document.querySelectorAll('.codespace-log-line-message'), (el) => el.textContent)).toEqual(['first page', 'second page']);
    expect(document.querySelector<HTMLDetailsElement>('.codespace-log-group')!.open).toBe(false);
    expect(document.querySelector<HTMLElement>('[data-codespace-log-loading]')!.classList.contains('tw-hidden')).toBe(true);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log yields while rendering a large page', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    const lines = Array.from({length: 501}, (_, index) => ({timestamp: 1785037200 + index, message: `line ${index + 1}`}));
    vi.stubGlobal('fetch', vi.fn().mockResolvedValue(Response.json({next_offset: 501, eof: true, operation_active: false, lines})));

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    expect(document.querySelectorAll('.codespace-log-line')).toHaveLength(500);

    await vi.advanceTimersByTimeAsync(16);
    expect(document.querySelectorAll('.codespace-log-line')).toHaveLength(501);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log confirms an inactive EOF once and then stops polling', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    const fetchMock = vi.fn().mockImplementation(() => Promise.resolve(Response.json({next_offset: 12, eof: true, operation_active: false, lines: []})));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(10);
    await vi.advanceTimersByTimeAsync(100);

    expect(fetchMock).toHaveBeenCalledTimes(2);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace log backs off when a non-EOF response does not advance', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML('12');
    const fetchMock = vi.fn().mockResolvedValue(Response.json({next_offset: 12, eof: false, operation_active: true, lines: []}));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenCalledTimes(1);

    await vi.advanceTimersByTimeAsync(10);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(10);
    expect(fetchMock).toHaveBeenCalledTimes(2);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});
