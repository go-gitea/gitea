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
  // Polling tests must not depend on which browser test page has focus.
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  document.documentElement.lang = 'en-US';
});

afterEach(() => {
  vi.restoreAllMocks();
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
      <div data-codespace-environment-detail="large" class="tw-hidden"></div>
    </form>`;

  initCodespaceCreateForm();
  document.querySelector<HTMLElement>('[data-value="large"]')!.click();

  expect(document.querySelector<HTMLInputElement>('input[name="environment_tag"]')!.value).toBe('large');
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-text]')!.textContent).toBe('large');
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="standard"]')!.classList.contains('tw-hidden')).toBe(true);
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="large"]')!.classList.contains('tw-hidden')).toBe(false);

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
      <input name="request_hash" value="reviewed">
      <input name="recommended_secret_value_TOKEN" value="private-value">
      <button type="button" data-codespace-review class="tw-hidden">Update configuration</button>
      <button data-codespace-create>Create</button>
    </form>`;

  initCodespaceCreateForm();
  const select = document.querySelector<HTMLSelectElement>('[data-codespace-dev-container]')!;
  select.value = '.devcontainer/node/devcontainer.json';
  select.dispatchEvent(new Event('change'));

  expect(navigations).toHaveLength(0);
  expect(document.querySelector<HTMLButtonElement>('[data-codespace-create]')!.disabled).toBe(true);
  expect(document.querySelector('[data-codespace-review]')!.classList.contains('tw-hidden')).toBe(false);
  document.querySelector<HTMLButtonElement>('[data-codespace-review]')!.click();

  const previewURL = new URL(navigations.at(-1)!.url);
  select.selectedIndex = 0;
  select.dispatchEvent(new Event('change'));
  expect(document.querySelector('[data-codespace-review]')!.classList.contains('tw-hidden')).toBe(true);
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

test('initCodespaceLiveState refreshes state and preserves expanded host verification', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = '<div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="10" data-log-revision="1:0"><span>old</span><details id="verification" open><summary>SSH</summary>fingerprint</details></div>';
    const fetchMock = vi.fn().mockResolvedValue(new Response(
      '<div id="codespace-live-state" data-state-url="/-/codespaces/uuid/state" data-refresh-after-ms="0" data-log-revision="2:20"><span>new</span><details id="verification"><summary>SSH</summary>fingerprint</details></div>',
      {status: 200},
    ));
    vi.stubGlobal('fetch', fetchMock);

    initCodespaceLiveState();
    const logView = document.createElement('div');
    logView.id = 'codespace-log-view';
    const wakeLog = vi.fn();
    logView.addEventListener('codespace-log-update', wakeLog);
    document.body.append(logView);
    await vi.advanceTimersByTimeAsync(10);
    await vi.waitFor(() => expect(document.querySelector('#codespace-live-state')!.textContent).toContain('new'));

    expect(fetchMock).toHaveBeenCalledWith('/-/codespaces/uuid/state', expect.objectContaining({method: 'GET'}));
    expect(document.querySelector<HTMLDetailsElement>('#verification')!.open).toBe(true);
    expect(wakeLog).toHaveBeenCalledTimes(1);
  } finally {
    vi.useRealTimers();
    vi.unstubAllGlobals();
    document.body.replaceChildren();
  }
});

test('codespace list refresh recovers from body failures and stops when access is lost', {concurrent: false}, async () => {
  try {
    vi.useFakeTimers();
    document.body.innerHTML = `
      <div data-codespace-state-error class="tw-hidden"><span data-codespace-state-interrupted></span><span data-codespace-state-unavailable class="tw-hidden"></span><button data-codespace-state-retry>Retry</button></div>
      <div id="codespace-list-state" data-state-url="/-/codespaces?owner=org&page=2&partial=true" data-refresh-after-ms="10">Resuming</div>`;
    const failed = new Response();
    vi.spyOn(failed, 'text').mockRejectedValue(new Error('body read failed'));
    const ready = new Response();
    vi.spyOn(ready, 'text').mockResolvedValue('<div id="codespace-list-state" data-state-url="/-/codespaces?owner=org&page=2&partial=true" data-refresh-after-ms="10">Running</div>');
    const fetchMock = vi.fn().mockResolvedValue(failed);
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(70);
    expect(document.querySelector<HTMLElement>('[data-codespace-state-error]')!.classList.contains('tw-hidden')).toBe(false);
    fetchMock.mockResolvedValueOnce(ready).mockResolvedValue(new Response('', {status: 404}));
    document.querySelector<HTMLButtonElement>('[data-codespace-state-retry]')!.click();
    await vi.advanceTimersByTimeAsync(0);
    expect(document.querySelector('#codespace-list-state')!.textContent).toBe('Running');
    expect(document.querySelector<HTMLElement>('[data-codespace-state-error]')!.classList.contains('tw-hidden')).toBe(true);
    await vi.advanceTimersByTimeAsync(10);
    expect(document.querySelector<HTMLElement>('[data-codespace-state-unavailable]')!.classList.contains('tw-hidden')).toBe(false);
    const calls = fetchMock.mock.calls.length;
    await vi.advanceTimersByTimeAsync(30000);
    expect(fetchMock).toHaveBeenCalledTimes(calls);
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
      <div data-auto-stop-unavailable class="tw-hidden"></div>
      <div data-auto-stop-out-of-range class="tw-hidden"></div>
      <fieldset data-auto-stop-fields>
        <input type="radio" name="mode" value="default">
        <input type="radio" name="mode" value="custom">
        <input type="radio" name="mode" value="never">
        <div data-auto-stop-custom-fields class="tw-hidden">
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
  expect(form.querySelector<HTMLElement>('[data-auto-stop-custom-fields]')!.classList.contains('tw-hidden')).toBe(false);
  expect(form.querySelector<HTMLElement>('[data-auto-stop-effective]')!.textContent).toBe('Stop after two hours');
  expect(showFomanticModal).toHaveBeenCalledWith(form);
  document.body.replaceChildren();
});

function codespaceLogHTML() {
  return `
    <section class="codespace-log-panel">
      <span class="tw-invisible" data-codespace-log-loading>Loading</span>
      <div id="codespace-log-view" data-log-url="/-/codespaces/1/logs" data-log-next-offset="0" data-log-refresh-after-ms="10">
        <div data-log-empty-message>Empty</div>
        <pre data-log-content></pre>
      </div>
      <div class="tw-hidden" data-log-error>Log unavailable<button data-log-retry>Retry</button></div>
    </section>
  `;
}

function codespaceLogResponse(nextOffset: number, messages: string[] = [], eof = true, active = false) {
  const response = new Response();
  // Native response body reads run outside fake timers.
  vi.spyOn(response, 'json').mockResolvedValue({
    next_offset: nextOffset, eof, operation_active: active,
    lines: messages.map((message) => ({timestamp: 1785037200, message})),
  });
  return response;
}

describe('codespace log', () => {
  let logView: HTMLElement;
  let content: HTMLElement;
  let height: number;

  beforeEach(() => {
    vi.useFakeTimers();
    document.body.innerHTML = codespaceLogHTML();
    logView = document.querySelector<HTMLElement>('#codespace-log-view')!;
    content = logView.querySelector<HTMLElement>('[data-log-content]')!;
    height = 400;
    Object.defineProperties(logView, {
      clientHeight: {value: 100},
      scrollHeight: {get: () => height, configurable: true},
      scrollTop: {value: 0, writable: true},
    });
  });

  afterEach(() => {
    window.dispatchEvent(new PageTransitionEvent('pagehide'));
    document.body.replaceChildren();
    vi.useRealTimers();
    vi.unstubAllGlobals();
  });

  test('loads history only on demand and uses the server byte offset', async () => {
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(101, ['中文'], false))
      .mockResolvedValueOnce(codespaceLogResponse(205, ['next page']));
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(content.textContent).toBe('中文\n');
    expect(logView.scrollTop).toBe(0);
    logView.scrollTop = 300;
    logView.dispatchEvent(new Event('scroll'));
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/-/codespaces/1/logs?offset=101&limit=65536', expect.objectContaining({method: 'GET', signal: expect.any(AbortSignal)}));
    expect(content.textContent).toBe('中文\nnext page\n');
    expect(logView.scrollTop).toBe(300);
  });

  test('fills a short viewport and renders untrusted output as plain text', async () => {
    height = 50;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(20, ['\u001b[32mready\u001b[0m'], false))
      .mockImplementationOnce(() => {
        height = 400;
        return Promise.resolve(codespaceLogResponse(100, ['<img src=x onerror=alert(1)>', '\u001b]8;;https://example.com\u0007link\u001b]8;;\u0007']));
      });
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(content.textContent).toBe('ready\n<img src=x onerror=alert(1)>\nlink\n');
    expect(content.childElementCount).toBe(0);
  });

  test('follows the live tail but preserves the position if the reader scrolls up', async () => {
    logView.scrollTop = 300;
    Object.defineProperty(logView, 'scrollHeight', {get: () => content.textContent.includes('second') ? 500 : 400});
    const page = Promise.withResolvers<Response>();
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(10, ['first'], true, true))
      .mockResolvedValueOnce(codespaceLogResponse(20, ['second'], true, true))
      .mockReturnValueOnce(page.promise);
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(10);
    expect(logView.scrollTop).toBe(500);
    // Match the browser's clamped scroll position after the mocked scrollHeight assignment.
    logView.scrollTop = 400;
    logView.dispatchEvent(new Event('scroll'));
    await vi.advanceTimersByTimeAsync(10);
    logView.scrollTop = 100;
    page.resolve(codespaceLogResponse(30, ['third'], true, true));
    await vi.advanceTimersByTimeAsync(0);
    expect(content.textContent).toBe('first\nsecond\nthird\n');
    expect(logView.scrollTop).toBe(100);
    await vi.advanceTimersByTimeAsync(100);
    expect(fetchMock).toHaveBeenCalledTimes(3);
  });

  test('confirms final EOF and wakes when the state fragment reports new output', async () => {
    height = 50;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(10, ['finished']))
      .mockResolvedValueOnce(codespaceLogResponse(10))
      .mockResolvedValue(codespaceLogResponse(20, ['resume started'], true, true));
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(100);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    logView.dispatchEvent(new Event('codespace-log-update'));
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(content.textContent).toBe('finished\nresume started\n');
  });

  test('backs off on no progress and pauses while the page is hidden', async () => {
    height = 50;
    const fetchMock = vi.fn().mockResolvedValue(codespaceLogResponse(0, [], false, true));
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(19);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('hidden');
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(1000);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
    document.dispatchEvent(new Event('visibilitychange'));
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenCalledTimes(3);
    expect(logView.getAttribute('data-log-next-offset')).toBe('0');
  });

  test.each([500, 404, 409])('preserves loaded text when a request fails with %i', async (status) => {
    height = 50;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(10, ['diagnostic'], true, true))
      .mockResolvedValueOnce(new Response(null, {status}))
      .mockResolvedValue(codespaceLogResponse(20, ['recovered'], true, true));
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(10);
    expect(content.textContent).toBe('diagnostic\n');
    expect(document.querySelector('[data-log-error]')!.classList.contains('tw-hidden')).toBe(false);
    const retry = document.querySelector<HTMLButtonElement>('[data-log-retry]')!;
    if (status === 500) {
      retry.click();
      await vi.advanceTimersByTimeAsync(0);
      expect(content.textContent).toBe('diagnostic\nrecovered\n');
      expect(document.querySelector('[data-log-error]')!.classList.contains('tw-hidden')).toBe(true);
    } else {
      expect(retry.disabled).toBe(true);
      await vi.advanceTimersByTimeAsync(1000);
      expect(fetchMock).toHaveBeenCalledTimes(2);
    }
  });

  test('serializes requests and aborts on navigation', async () => {
    height = 50;
    const page = Promise.withResolvers<Response>();
    const fetchMock = vi.fn().mockReturnValue(page.promise);
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    initCodespaceLiveState();
    logView.dispatchEvent(new Event('scroll'));
    logView.dispatchEvent(new Event('codespace-log-update'));
    await vi.advanceTimersByTimeAsync(100);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const signal = fetchMock.mock.calls[0][1].signal as AbortSignal;
    window.dispatchEvent(new PageTransitionEvent('pagehide'));
    expect(signal.aborted).toBe(true);
    page.resolve(codespaceLogResponse(10, ['late']));
    await vi.advanceTimersByTimeAsync(100);
    expect(content.textContent).toBe('');
    expect(fetchMock).toHaveBeenCalledTimes(1);
  });

  test('continues from the rendered offset after browser history restores the page', async () => {
    height = 50;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(10, ['before']))
      .mockResolvedValueOnce(codespaceLogResponse(20, ['after']));
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(0);
    window.dispatchEvent(new PageTransitionEvent('pagehide', {persisted: true}));
    window.dispatchEvent(new PageTransitionEvent('pageshow', {persisted: true}));
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/-/codespaces/1/logs?offset=10&limit=65536', expect.anything());
    expect(content.textContent).toBe('before\nafter\n');
  });
});
