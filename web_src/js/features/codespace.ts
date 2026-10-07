import {hideElem, showElem, toggleElem} from '../utils/dom.ts';
import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {hideFomanticModal, showFomanticModal} from '../modules/fomantic/modal.ts';
import stripAnsi from 'strip-ansi';
import {Idiomorph} from 'idiomorph';
import {ignoreAreYouSure} from '../modules/are-you-sure.ts';

const liveStateSelector = '#codespace-live-state, #codespace-list-state';
const logViewSelector = '#codespace-log-view';
const maxRefreshBackoff = 30000;

type CodespaceLogLine = {
  timestamp: number;
  message: string;
};

const stateRefreshTimers = new WeakMap<HTMLElement, ReturnType<typeof setTimeout>>();
const initializedSettingsButtons = new WeakSet<HTMLElement>();

export function initCodespaceCreateForm() {
  const form = document.querySelector<HTMLFormElement>('[data-codespace-create-form]');
  if (!form) return;

  const environmentInput = form.querySelector<HTMLInputElement>('input[name="environment_tag"]');
  const environmentDropdown = environmentInput?.closest<HTMLElement>('.dropdown');
  const environmentText = environmentDropdown?.querySelector<HTMLElement>('[data-codespace-environment-text]');
  const environmentDetails = form.querySelectorAll<HTMLElement>('[data-codespace-environment-detail]');
  const selectEnvironment = (tag: string) => {
    if (environmentInput) environmentInput.value = tag;
    if (environmentText && tag) {
      environmentText.textContent = tag;
      environmentText.classList.remove('default');
    }
    environmentDropdown?.classList.remove('error');
    environmentDropdown?.removeAttribute('aria-invalid');
    for (const detail of environmentDetails) {
      toggleElem(detail, detail.getAttribute('data-codespace-environment-detail') === tag);
    }
  };
  environmentInput?.addEventListener('change', () => {
    selectEnvironment(environmentInput.value);
  });
  for (const option of form.querySelectorAll<HTMLElement>('.codespace-create-environment-option')) {
    option.addEventListener('click', () => {
      selectEnvironment(option.getAttribute('data-value') ?? '');
    });
  }
  form.addEventListener('submit', (event) => {
    if (form.querySelector<HTMLButtonElement>('[data-codespace-create]')?.disabled) {
      event.preventDefault();
      return;
    }
    if (environmentInput?.value) return;
    event.preventDefault();
    environmentDropdown?.classList.add('error');
    environmentDropdown?.setAttribute('aria-invalid', 'true');
    environmentDropdown?.focus();
  });

  const devContainerSelect = form.querySelector<HTMLSelectElement>('[data-codespace-dev-container]');
  const reviewButton = form.querySelector<HTMLButtonElement>('[data-codespace-review]');
  const createButton = form.querySelector<HTMLButtonElement>('[data-codespace-create]');
  const reviewedSelection = devContainerSelect?.value;
  devContainerSelect?.addEventListener('change', () => {
    const changed = devContainerSelect.value !== reviewedSelection;
    if (reviewButton) toggleElem(reviewButton, changed);
    const warning = form.querySelector<HTMLElement>('[data-codespace-review-warning]');
    if (warning) toggleElem(warning, changed);
    if (createButton) createButton.disabled = changed || !form.querySelector<HTMLInputElement>('[name="request_hash"]')!.value;
  });
  reviewButton?.addEventListener('click', () => {
    if (!devContainerSelect) return;
    const previewURL = new URL(devContainerSelect.getAttribute('data-preview-url')!, window.location.href);
    previewURL.searchParams.set('ref_type', devContainerSelect.getAttribute('data-ref-type')!);
    previewURL.searchParams.set('ref_name', devContainerSelect.getAttribute('data-ref-name')!);
    previewURL.searchParams.set('dev_container', devContainerSelect.value);
    if (environmentInput?.value) previewURL.searchParams.set('environment_tag', environmentInput.value);
    ignoreAreYouSure(form);
    window.location.assign(previewURL);
  });
}

export function initCodespaceLiveState() {
  initCodespaceOpenModal();
  initCodespaceSettingsModal();
  const openModalEl = document.querySelector<HTMLElement>('#codespace-open-modal[data-codespace-auto-open="true"]');
  if (openModalEl) showFomanticModal(openModalEl);

  const stateEl = document.querySelector<HTMLElement>(liveStateSelector);
  if (stateEl) scheduleCodespaceStateRefresh(stateEl, 0);
  document.querySelector('[data-codespace-state-retry]')?.addEventListener('click', () => {
    const current = document.querySelector<HTMLElement>(liveStateSelector);
    if (!current) return;
    clearTimeout(stateRefreshTimers.get(current));
    refreshCodespaceState(current, 0);
  });

  const logEl = document.querySelector<HTMLElement>(logViewSelector);
  if (logEl) {
    initCodespaceLog(logEl);
  }
}

function initCodespaceOpenModal() {
  const form = document.querySelector<HTMLFormElement>('#codespace-open-modal');
  if (!form || form.getAttribute('data-codespace-initialized') === 'true') return;
  form.setAttribute('data-codespace-initialized', 'true');
  form.addEventListener('submit', () => {
    if (form.target !== '_blank') return;
    setTimeout(() => {
      form.classList.remove('is-loading');
      hideFomanticModal(form);
    }, 0);
  });
}

function initCodespaceSettingsModal() {
  const form = document.querySelector<HTMLFormElement>('#codespace-settings-modal');
  if (!form || form.getAttribute('data-codespace-initialized') === 'true') return;
  form.setAttribute('data-codespace-initialized', 'true');
  for (const radio of form.querySelectorAll<HTMLInputElement>('input[name="mode"]')) {
    radio.addEventListener('change', () => syncCodespaceAutoStopFields(form, true));
  }
  initCodespaceSettingsButtons(document, form);
}

function initCodespaceSettingsButtons(root: ParentNode, form: HTMLFormElement) {
  for (const button of root.querySelectorAll<HTMLElement>('.codespace-settings-button')) {
    if (initializedSettingsButtons.has(button)) continue;
    initializedSettingsButtons.add(button);
    button.addEventListener('click', () => {
      form.action = button.getAttribute('data-settings-url')!;
      form.querySelector<HTMLInputElement>('input[name="return_to"]')!.value = button.getAttribute('data-return-to')!;
      form.querySelector<HTMLElement>('[data-auto-stop-effective]')!.textContent = button.getAttribute('data-auto-stop-effective')!;
      form.querySelector<HTMLElement>('[data-auto-stop-default-description]')!.textContent = button.getAttribute('data-auto-stop-default-description')!;
      form.querySelector<HTMLElement>('[data-auto-stop-range]')!.textContent = button.getAttribute('data-auto-stop-range')!;
      const mode = button.getAttribute('data-auto-stop-mode')!;
      for (const radio of form.querySelectorAll<HTMLInputElement>('input[name="mode"]')) radio.checked = radio.value === mode;
      form.querySelector<HTMLInputElement>('input[name="timeout_value"]')!.value = button.getAttribute('data-auto-stop-timeout-value')!;
      fomanticQuery(form.querySelector<HTMLSelectElement>('select[name="timeout_unit"]')!).dropdown('set selected', button.getAttribute('data-auto-stop-timeout-unit')!);
      toggleElem(form.querySelector<HTMLElement>('[data-auto-stop-out-of-range]')!, button.getAttribute('data-auto-stop-out-of-range') === 'true');
      setAutoStopAvailability(form, button.getAttribute('data-auto-stop-configurable') === 'true');
      syncCodespaceAutoStopFields(form, false);
      showFomanticModal(form);
    });
  }
}

function setAutoStopAvailability(form: HTMLFormElement, configurable: boolean) {
  form.querySelector<HTMLFieldSetElement>('[data-auto-stop-fields]')!.disabled = !configurable;
  toggleElem(form.querySelector<HTMLElement>('[data-auto-stop-unavailable]')!, !configurable);
  form.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled = !configurable;
}

function syncCodespaceAutoStopFields(form: HTMLFormElement, focus: boolean) {
  const customFields = form.querySelector<HTMLElement>('[data-auto-stop-custom-fields]')!;
  const timeoutInput = customFields.querySelector<HTMLInputElement>('input[name="timeout_value"]')!;
  const timeoutUnit = customFields.querySelector<HTMLSelectElement>('select[name="timeout_unit"]')!;
  const customSelected = form.querySelector<HTMLInputElement>('input[name="mode"]:checked')!.value === 'custom';
  toggleElem(customFields, customSelected);
  const enabled = customSelected && !form.querySelector<HTMLFieldSetElement>('[data-auto-stop-fields]')!.disabled;
  timeoutInput.disabled = !enabled;
  timeoutUnit.disabled = !enabled;
  const timeoutDropdown = timeoutUnit.parentElement!.classList.contains('dropdown') ? timeoutUnit.parentElement! : timeoutUnit;
  timeoutDropdown.classList.toggle('disabled', !enabled);
  timeoutDropdown.setAttribute('aria-disabled', String(!enabled));
  if (enabled && focus) timeoutInput.focus();
}

function scheduleCodespaceStateRefresh(stateEl: HTMLElement, failureCount: number) {
  const refreshAfter = Number(stateEl.getAttribute('data-refresh-after-ms'));
  if (!Number.isFinite(refreshAfter) || refreshAfter <= 0) return;
  stateRefreshTimers.set(stateEl, setTimeout(() => {
    if (document.visibilityState === 'hidden') {
      waitForVisible(() => scheduleCodespaceStateRefresh(stateEl, failureCount));
      return;
    }
    refreshCodespaceState(stateEl, failureCount);
  }, refreshDelay(refreshAfter, failureCount)));
}

async function refreshCodespaceState(stateEl: HTMLElement, failureCount: number) {
  if (!stateEl.isConnected || stateEl.getAttribute('data-refreshing') === 'true') return;
  const stateUrl = stateEl.getAttribute('data-state-url');
  if (!stateUrl) return;
  if (stateEl.querySelector('.dropdown.active, .dropdown.visible, input:focus, textarea:focus, select:focus, [contenteditable]:focus')) {
    scheduleCodespaceStateRefresh(stateEl, failureCount);
    return;
  }
  stateEl.setAttribute('data-refreshing', 'true');
  const notice = document.querySelector<HTMLElement>('[data-codespace-state-error]');
  let nextStateEl: HTMLElement | null;
  let terminalFailure = false;
  try {
    const response = await GET(stateUrl);
    terminalFailure = response.status === 403 || response.status === 404;
    if (!response.ok) throw new Error('State refresh failed');
    const nextDocument = new DOMParser().parseFromString(await response.text(), 'text/html');
    nextStateEl = nextDocument.querySelector<HTMLElement>(liveStateSelector);
    if (!nextStateEl) {
      terminalFailure = response.redirected;
      throw new Error('State fragment is unavailable');
    }
  } catch {
    if (notice && (terminalFailure || failureCount >= 2)) {
      showElem(notice);
      toggleElem(notice.querySelector<HTMLElement>('[data-codespace-state-retry]')!, !terminalFailure);
      toggleElem(notice.querySelector<HTMLElement>('[data-codespace-state-unavailable]')!, terminalFailure);
      toggleElem(notice.querySelector<HTMLElement>('[data-codespace-state-interrupted]')!, !terminalFailure);
    }
    if (!terminalFailure) scheduleCodespaceStateRefresh(stateEl, failureCount + 1);
    return;
  } finally {
    stateEl.removeAttribute('data-refreshing');
  }
  if (notice) hideElem(notice);
  if (stateEl.querySelector('.dropdown.active, .dropdown.visible, input:focus, textarea:focus, select:focus, [contenteditable]:focus')) {
    scheduleCodespaceStateRefresh(stateEl, 0);
    return;
  }

  const detailPage = stateEl.closest<HTMLElement>('.codespace-detail-page');
  const currentMode = stateEl.getAttribute('data-detail-mode');
  const nextMode = nextStateEl.getAttribute('data-detail-mode');
  const explicitTab = detailPage?.getAttribute('data-codespace-tab-explicit') === 'true';
  if (!explicitTab && currentMode && nextMode && currentMode !== nextMode && !document.querySelector('.ui.modal.visible')) {
    window.location.reload();
    return;
  }

  // Keep disclosure state while updating the server-owned view.
  for (const details of stateEl.querySelectorAll<HTMLDetailsElement>('details[id]')) {
    const nextDetails = nextStateEl.querySelector<HTMLDetailsElement>(`#${CSS.escape(details.id)}`);
    if (nextDetails) nextDetails.open = details.open;
  }
  const logRevision = stateEl.getAttribute('data-log-revision');
  Idiomorph.morph(stateEl, nextStateEl, {morphStyle: 'outerHTML'});
  const currentStateEl = document.querySelector<HTMLElement>(liveStateSelector)!;
  if (logRevision !== currentStateEl.getAttribute('data-log-revision')) {
    document.querySelector(logViewSelector)?.dispatchEvent(new Event('codespace-log-update'));
  }
  const settingsForm = document.querySelector<HTMLFormElement>('#codespace-settings-modal');
  if (settingsForm) initCodespaceSettingsButtons(currentStateEl, settingsForm);
  if (settingsForm?.classList.contains('visible')) {
    const configurable = currentStateEl.id === 'codespace-live-state' ?
      currentStateEl.getAttribute('data-auto-stop-configurable') === 'true' :
      Array.from(currentStateEl.querySelectorAll<HTMLElement>('.codespace-settings-button')).some((button) => new URL(button.getAttribute('data-settings-url')!, window.location.href).href === settingsForm.action);
    setAutoStopAvailability(settingsForm, configurable);
    syncCodespaceAutoStopFields(settingsForm, false);
  }
  scheduleCodespaceStateRefresh(currentStateEl, 0);
}

function initCodespaceLog(logEl: HTMLElement) {
  if (logEl.getAttribute('data-log-initialized') === 'true') return;
  logEl.setAttribute('data-log-initialized', 'true');
  const panel = logEl.closest<HTMLElement>('.codespace-log-panel')!;
  const content = logEl.querySelector<HTMLElement>('[data-log-content]')!;
  const empty = logEl.querySelector<HTMLElement>('[data-log-empty-message]')!;
  const loading = panel.querySelector<HTMLElement>('[data-codespace-log-loading]')!;
  const error = panel.querySelector<HTMLElement>('[data-log-error]')!;
  const retry = panel.querySelector<HTMLButtonElement>('[data-log-retry]')!;
  const lifetime = new AbortController();
  const refreshAfter = Number(logEl.getAttribute('data-log-refresh-after-ms'));
  const logURL = logEl.getAttribute('data-log-url')!;
  let offset = Number(logEl.getAttribute('data-log-next-offset'));
  let eof = false;
  let settled = false;
  let pending = false;
  let failures = 0;
  let revision = 0;
  let inactiveOffset: number | undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;

  const atBottom = () => logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight <= 32;
  const schedule = (delay: number) => {
    clearTimeout(timer);
    if (!settled && logEl.isConnected && document.visibilityState !== 'hidden') {
      timer = setTimeout(load, delay);
    }
  };
  async function load() {
    if (pending || settled || !logEl.isConnected || lifetime.signal.aborted || document.visibilityState === 'hidden') return;
    clearTimeout(timer);
    timer = undefined;
    pending = true;
    loading.classList.remove('tw-invisible');
    const requestRevision = revision;
    const following = eof && atBottom();
    try {
      const response = await GET(`${logURL}?offset=${offset}&limit=65536`, {signal: lifetime.signal});
      if (!response.ok || response.redirected) {
        settled = response.redirected || response.status === 401 || response.status === 403 || response.status === 404 || response.status === 409;
        throw new Error('Log request failed');
      }
      const result: {lines: CodespaceLogLine[], next_offset: number, eof: boolean, operation_active: boolean} = await response.json();
      if (!logEl.isConnected || lifetime.signal.aborted) return;
      if (!Array.isArray(result.lines) || !Number.isSafeInteger(result.next_offset) || result.next_offset < offset ||
          typeof result.eof !== 'boolean' || typeof result.operation_active !== 'boolean' ||
          result.lines.some((line) => typeof line.message !== 'string') ||
          (result.lines.length > 0 && result.next_offset === offset) || (!result.eof && result.next_offset === offset)) {
        throw new Error('Log offset did not advance');
      }

      if (result.lines.length > 0) {
        const followNewLines = following && atBottom();
        // One text node per page keeps large logs cheap without interpreting log commands or HTML.
        content.append(result.lines.map((line) => `${stripAnsi(line.message)}\n`).join(''));
        hideElem(empty);
        if (followNewLines) logEl.scrollTop = logEl.scrollHeight;
      }
      offset = result.next_offset;
      logEl.setAttribute('data-log-next-offset', String(offset));
      eof = result.eof;
      failures = 0;
      hideElem(error);
      if (eof && !result.operation_active && requestRevision === revision) {
        // State transitions may commit before their final diagnostic line is appended.
        settled = inactiveOffset === offset;
        inactiveOffset = offset;
      } else {
        inactiveOffset = undefined;
      }

      if (!eof) {
        if (logEl.scrollHeight <= logEl.clientHeight) schedule(0);
      } else if (atBottom()) {
        schedule(refreshAfter);
      }
    } catch {
      if (lifetime.signal.aborted || !logEl.isConnected) return;
      showElem(error);
      retry.disabled = settled;
      if (atBottom()) schedule(refreshDelay(refreshAfter, ++failures));
    } finally {
      pending = false;
      loading.classList.add('tw-invisible');
    }
  }

  logEl.addEventListener('scroll', () => {
    if (!atBottom()) {
      clearTimeout(timer);
      timer = undefined;
    } else if (!pending && timer === undefined) {
      schedule(0);
    }
  }, {signal: lifetime.signal});
  retry.addEventListener('click', () => {
    if (!pending) schedule(0);
  }, {signal: lifetime.signal});
  logEl.addEventListener('codespace-log-update', () => {
    revision++;
    settled = false;
    inactiveOffset = undefined;
    if (atBottom() && !pending) schedule(0);
  }, {signal: lifetime.signal});
  document.addEventListener('visibilitychange', () => {
    clearTimeout(timer);
    timer = undefined;
    if (document.visibilityState !== 'hidden' && atBottom()) schedule(0);
  }, {signal: lifetime.signal});
  window.addEventListener('pagehide', (event) => {
    clearTimeout(timer);
    lifetime.abort();
    logEl.removeAttribute('data-log-initialized');
    if (event.persisted) {
      window.addEventListener('pageshow', () => initCodespaceLog(logEl), {once: true});
    }
  }, {once: true, signal: lifetime.signal});
  load();
}

function refreshDelay(baseDelay: number, failureCount: number) {
  if (failureCount <= 0) return baseDelay;
  return Math.min(maxRefreshBackoff, baseDelay * 2 ** Math.min(failureCount, 4));
}

function waitForVisible(callback: () => void) {
  const onVisible = () => {
    if (document.visibilityState === 'hidden') return;
    document.removeEventListener('visibilitychange', onVisible);
    callback();
  };
  document.addEventListener('visibilitychange', onVisible);
}
