import '../../fomantic/build/fomantic.js';
import {initCodespaceCreateForm, initCodespaceLiveState} from './codespace.ts';
import {hideModal, showModal} from '../modules/modal.ts';
import {captureNavigations} from '../utils/testhelper.ts';
import {initGiteaFomantic} from '../modules/fomantic.ts';
import {userEvent} from 'vitest/browser';

vi.mock('../modules/modal.ts', () => ({hideModal: vi.fn(), showModal: vi.fn()}));
beforeAll(() => initGiteaFomantic());

beforeEach(() => {
  vi.clearAllMocks();
  // Polling tests must not depend on which browser test page has focus.
  vi.spyOn(document, 'visibilityState', 'get').mockReturnValue('visible');
  document.documentElement.lang = 'en-US';
});

afterEach(() => {
  window.dispatchEvent(new PageTransitionEvent('pagehide'));
  for (const dropdown of document.querySelectorAll('.ui.dropdown')) $(dropdown).data('module-dropdown')?.destroy();
  document.body.replaceChildren();
  vi.useRealTimers();
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

test('codespace create environment selection updates its explanation', () => {
  document.body.innerHTML = `
    <form data-codespace-create-form>
      <div class="dropdown">
        <input name="environment_tag" value="standard">
      </div>
      <div data-codespace-environment-detail="standard"></div>
      <div data-codespace-environment-detail="large" class="tw-hidden"></div>
    </form>`;

  initCodespaceCreateForm();
  const environmentInput = document.querySelector<HTMLInputElement>('input[name="environment_tag"]')!;
  environmentInput.value = 'large';
  environmentInput.dispatchEvent(new Event('change'));

  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="standard"]')!.classList.contains('tw-hidden')).toBe(true);
  expect(document.querySelector<HTMLElement>('[data-codespace-environment-detail="large"]')!.classList.contains('tw-hidden')).toBe(false);

  environmentInput.value = '';
  const submitEvent = new SubmitEvent('submit', {cancelable: true});
  document.querySelector<HTMLFormElement>('form')!.dispatchEvent(submitEvent);
  expect(submitEvent.defaultPrevented).toBe(true);
  expect(document.querySelector('.dropdown')!.classList.contains('error')).toBe(true);
  expect(document.querySelector('.dropdown')!.getAttribute('aria-invalid')).toBe('true');
});

test.each([false, true])('codespace configuration preview preserves an explicitly selected environment: %s', (explicit) => {
  const navigations = captureNavigations();
  document.body.innerHTML = `
    <form data-codespace-create-form>
      <input name="environment_tag" value="standard">
      <input type="hidden" data-codespace-dev-container data-preview-url="/owner/repo/codespaces/new"
        data-ref-type="branch" data-ref-name="main" value=".devcontainer/devcontainer.json">
      <input name="request_hash" value="reviewed">
      <input name="secret_value" value="private-value">
      <button type="button" data-codespace-review class="tw-hidden">Update configuration</button>
      <button data-codespace-create>Create</button>
    </form>`;

  initCodespaceCreateForm();
  if (explicit) document.querySelector('[name="environment_tag"]')!.dispatchEvent(new Event('change'));
  const input = document.querySelector<HTMLInputElement>('[data-codespace-dev-container]')!;
  input.value = '.devcontainer/node/devcontainer.json';
  input.dispatchEvent(new Event('change'));

  expect(navigations).toHaveLength(0);
  expect(document.querySelector<HTMLButtonElement>('[data-codespace-create]')!.disabled).toBe(true);
  expect(document.querySelector('[data-codespace-review]')!.classList.contains('tw-hidden')).toBe(false);
  document.querySelector<HTMLButtonElement>('[data-codespace-review]')!.click();

  const previewURL = new URL(navigations.at(-1)!.url);
  input.value = '.devcontainer/devcontainer.json';
  input.dispatchEvent(new Event('change'));
  expect(document.querySelector('[data-codespace-review]')!.classList.contains('tw-hidden')).toBe(true);
  expect(previewURL.pathname).toBe('/owner/repo/codespaces/new');
  expect(Object.fromEntries(previewURL.searchParams)).toEqual({
    ref_type: 'branch',
    ref_name: 'main',
    dev_container: '.devcontainer/node/devcontainer.json',
    ...(explicit && {environment_tag: 'standard'}),
  });
});

test('codespace secret dropdowns select saved or multiline custom values and preserve row order', async () => {
  document.body.innerHTML = `
    <form class="ui form" data-codespace-create-form>
      <div data-codespace-secret-list>
        <div data-codespace-secret-row>
          <input name="secret_name" value="TOKEN" readonly>
          <select class="ui selection dropdown custom" name="secret_source"><option value="none" selected>Omit</option><option value="saved">Saved</option><option value="custom">Custom</option></select>
          <div data-codespace-secret-value class="tw-hidden"><textarea name="secret_value"></textarea></div>
        </div>
      </div>
      <template data-codespace-secret-template>
        <div data-codespace-secret-row>
          <select class="ui search selection dropdown custom" name="secret_name"><option value="">Select or enter a name</option><option value="TOKEN">TOKEN</option><option value="SECOND_TOKEN">SECOND_TOKEN</option></select>
          <select class="ui selection dropdown custom" name="secret_source"><option value="none" selected>Omit</option><option value="saved" disabled>Saved</option><option value="custom">Custom</option></select>
          <div data-codespace-secret-value class="tw-hidden"><textarea name="secret_value"></textarea></div>
          <button type="button" data-codespace-secret-remove>Remove</button>
        </div>
      </template>
      <button type="button" data-codespace-secret-add>Add</button>
    </form>`;
  initCodespaceCreateForm();
  const declaredSource = document.querySelector<HTMLSelectElement>('[name="secret_source"]')!;
  expect(declaredSource.value).toBe('none');
  $(declaredSource).dropdown('set selected', 'saved');
  await userEvent.click(document.querySelector<HTMLButtonElement>('[data-codespace-secret-add]')!);
  const row = document.querySelector('[data-codespace-secret-list]')!.lastElementChild!;
  const name = row.querySelector<HTMLSelectElement>('[name="secret_name"]')!;
  const source = row.querySelector<HTMLSelectElement>('[name="secret_source"]')!;
  const value = row.querySelector<HTMLTextAreaElement>('[name="secret_value"]')!;
  const search = row.querySelector<HTMLInputElement>('input.search')!;
  await expect.poll(() => document.activeElement === search).toBe(true);
  await userEvent.fill(search, 'token');
  await userEvent.click(name.closest('.dropdown')!.querySelector<HTMLElement>('.menu > [data-value="TOKEN"]')!);
  expect(name.value).toBe('TOKEN');
  expect(source.value).toBe('none');
  expect(source.querySelector<HTMLOptionElement>('[value="saved"]')!.disabled).toBe(false);
  await userEvent.click(source.closest('.dropdown')!);
  await userEvent.click(source.closest('.dropdown')!.querySelector<HTMLElement>('.menu > [data-value="saved"]')!);
  expect(source.value).toBe('saved');
  expect(value.parentElement!.classList.contains('tw-hidden')).toBe(true);
  $(name).dropdown('set selected', 'SECOND_TOKEN');
  expect(source.value).toBe('none');
  $(source).dropdown('set selected', 'saved');
  await userEvent.fill(search, '<b>OTHER</b>');
  await expect.poll(() => name.closest('.dropdown')!.querySelector('.addition')?.textContent).toBe('<b>OTHER</b>');
  expect(name.closest('.dropdown')!.querySelector('.addition b')).toBeNull();
  await userEvent.fill(search, 'OTHER');
  await expect.poll(() => name.closest('.dropdown')!.querySelector('.addition[data-value="OTHER"]')).toBeTruthy();
  await userEvent.keyboard('{Enter}');
  expect(name.value).toBe('OTHER');
  expect(source.value).toBe('none');
  expect(source.querySelector<HTMLOptionElement>('[value="saved"]')!.disabled).toBe(true);
  $(source).dropdown('set selected', 'custom');
  expect(value.parentElement!.classList.contains('tw-hidden')).toBe(false);
  value.value = ' first\nsecond\n';
  const form = document.querySelector('form')!;
  expect(new FormData(form).getAll('secret_name')).toEqual(['TOKEN', 'OTHER']);
  expect(new FormData(form).getAll('secret_source')).toEqual(['saved', 'custom']);
  expect(new FormData(form).getAll('secret_value')).toEqual(['', ' first\nsecond\n']);
  $(source).dropdown('set selected', 'none');
  expect(value.parentElement!.classList.contains('tw-hidden')).toBe(true);
  expect(new FormData(form).getAll('secret_source')).toEqual(['saved', 'none']);
  expect(new FormData(form).getAll('secret_value')).toHaveLength(2);
  row.querySelector<HTMLButtonElement>('[data-codespace-secret-remove]')!.click();
  expect(Array.from(new FormData(form))).toEqual([['secret_name', 'TOKEN'], ['secret_source', 'saved'], ['secret_value', '']]);
});

test('gateway recovery opens the modal and closes it after opening a new tab', async () => {
  vi.useFakeTimers();
  document.body.innerHTML = '<dialog id="codespace-open-modal" data-codespace-auto-open="true"><form target="_blank" class="is-loading"></form></dialog>';
  const form = document.querySelector<HTMLFormElement>('#codespace-open-modal form')!;

  initCodespaceLiveState();
  expect(showModal).toHaveBeenCalledWith(form.closest('dialog'));
  form.dispatchEvent(new SubmitEvent('submit'));
  await vi.runAllTimersAsync();

  expect(hideModal).toHaveBeenCalledWith(form.closest('dialog'));
  expect(form.classList.contains('is-loading')).toBe(false);
});

test('initCodespaceLiveState refreshes state and preserves expanded host verification', async () => {
  vi.useFakeTimers();
  document.body.innerHTML = '<div id="codespace-live-state" data-state-url="/-/codespaces/1/state" data-refresh-after-ms="10" data-log-revision="1:0"><span>old</span><details id="verification" open><summary>SSH</summary>fingerprint</details></div>';
  const fetchMock = vi.fn().mockResolvedValue(new Response(
    '<div id="codespace-live-state" data-state-url="/-/codespaces/1/state" data-refresh-after-ms="0" data-log-revision="2:20"><span>new</span><details id="verification"><summary>SSH</summary>fingerprint</details></div>',
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

  expect(fetchMock).toHaveBeenCalledWith('/-/codespaces/1/state', expect.objectContaining({method: 'GET'}));
  expect(document.querySelector<HTMLDetailsElement>('#verification')!.open).toBe(true);
  expect(wakeLog).toHaveBeenCalledTimes(1);
});

test('codespace refresh disables open settings when the runtime stops accepting changes', async () => {
  vi.useFakeTimers();
  document.body.innerHTML = `
    <div id="codespace-live-state" data-state-url="/-/codespaces/1/state" data-refresh-after-ms="10"></div>
    <dialog id="codespace-settings-modal" open><form>
      <div data-auto-stop-unavailable class="tw-hidden"></div>
      <fieldset data-auto-stop-fields>
        <input type="radio" name="mode" value="custom" checked>
        <div data-auto-stop-custom-fields><input name="timeout_value"><select name="timeout_unit"></select></div>
      </fieldset><button type="submit">Save</button>
    </form></dialog>`;
  const response = new Response();
  vi.spyOn(response, 'text').mockResolvedValue('<div id="codespace-live-state" data-auto-stop-configurable="false"></div>');
  vi.stubGlobal('fetch', vi.fn().mockResolvedValue(response));
  initCodespaceLiveState();
  await vi.advanceTimersByTimeAsync(10);
  expect(document.querySelector<HTMLFieldSetElement>('[data-auto-stop-fields]')!.disabled).toBe(true);
  expect(document.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled).toBe(true);
});

test('codespace list refresh recovers from body failures and stops when access is lost', async () => {
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
});

test('codespace settings button fills and opens the shared auto-stop modal', () => {
  document.body.innerHTML = `
    <button class="codespace-settings-button"
      data-settings-url="/-/codespaces/1/auto-stop" data-return-to="/-/codespaces/1"
      data-auto-stop-mode="custom" data-auto-stop-timeout-value="2" data-auto-stop-timeout-unit="hours"
      data-auto-stop-configurable="true" data-auto-stop-out-of-range="false"
      data-auto-stop-effective="Stop after two hours" data-auto-stop-default-description="Site default" data-auto-stop-range="One minute to one day"></button>
    <dialog id="codespace-settings-modal"><form>
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
    </form></dialog>
  `;

  initCodespaceLiveState();
  document.querySelector<HTMLButtonElement>('.codespace-settings-button')!.click();

  const form = document.querySelector<HTMLFormElement>('#codespace-settings-modal form')!;
  expect(form.action).toMatch(/\/-\/codespaces\/1\/auto-stop$/);
  expect(Object.fromEntries(new FormData(form))).toEqual({
    return_to: '/-/codespaces/1', mode: 'custom', timeout_value: '2', timeout_unit: 'hours',
  });
  expect(form.querySelector<HTMLElement>('[data-auto-stop-custom-fields]')!.classList.contains('tw-hidden')).toBe(false);
  expect(showModal).toHaveBeenCalledWith(form.closest('dialog'));
});

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
    document.body.innerHTML = `
      <section class="codespace-log-panel">
        <div id="codespace-log-view" data-log-url="/-/codespaces/1/logs" data-log-next-offset="0" data-log-refresh-after-ms="10">
          <div data-log-empty-message>Empty</div>
          <pre data-log-content></pre>
        </div>
        <div class="tw-hidden" data-log-error>Log unavailable<button data-log-retry>Retry</button></div>
      </section>`;
    logView = document.querySelector<HTMLElement>('#codespace-log-view')!;
    content = logView.querySelector<HTMLElement>('[data-log-content]')!;
    height = 400;
    Object.defineProperties(logView, {
      clientHeight: {value: 100},
      scrollHeight: {get: () => height, configurable: true},
      scrollTop: {value: 0, writable: true},
    });
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
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] 中文\n');
    expect(logView.scrollTop).toBe(0);
    logView.scrollTop = 300;
    logView.dispatchEvent(new Event('scroll'));
    await vi.advanceTimersByTimeAsync(0);
    expect(fetchMock).toHaveBeenNthCalledWith(2, '/-/codespaces/1/logs?offset=101&limit=65536', expect.objectContaining({method: 'GET', signal: expect.any(AbortSignal)}));
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] 中文\n[2026-07-26T03:40:00.000Z] next page\n');
    expect(logView.scrollTop).toBe(300);
  });

  test('fills a short viewport and renders untrusted output as plain text', async () => {
    height = 50;
    const fetchMock = vi.fn()
      .mockResolvedValueOnce(codespaceLogResponse(20, ['ready'], false))
      .mockImplementationOnce(() => {
        height = 400;
        return Promise.resolve(codespaceLogResponse(100, ['<img src=x onerror=alert(1)>', 'link']));
      });
    vi.stubGlobal('fetch', fetchMock);
    initCodespaceLiveState();
    await vi.advanceTimersByTimeAsync(1);
    expect(fetchMock).toHaveBeenCalledTimes(2);
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] ready\n[2026-07-26T03:40:00.000Z] <img src=x onerror=alert(1)>\n[2026-07-26T03:40:00.000Z] link\n');
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
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] first\n[2026-07-26T03:40:00.000Z] second\n[2026-07-26T03:40:00.000Z] third\n');
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
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] finished\n[2026-07-26T03:40:00.000Z] resume started\n');
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
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] diagnostic\n');
    expect(document.querySelector('[data-log-error]')!.classList.contains('tw-hidden')).toBe(false);
    const retry = document.querySelector<HTMLButtonElement>('[data-log-retry]')!;
    if (status === 500) {
      retry.click();
      await vi.advanceTimersByTimeAsync(0);
      expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] diagnostic\n[2026-07-26T03:40:00.000Z] recovered\n');
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
    expect(content.textContent).toBe('[2026-07-26T03:40:00.000Z] before\n[2026-07-26T03:40:00.000Z] after\n');
  });
});
