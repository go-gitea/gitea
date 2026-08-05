import {GET} from '../modules/fetch.ts';
import {fomanticQuery} from '../modules/fomantic/base.ts';
import {hideFomanticModal, showFomanticModal} from '../modules/fomantic/modal.ts';
import {toggleFullScreen} from '../utils.ts';
import {formatDatetime} from '../utils/time.ts';
import {createLogLineMessage, decodeLogLineMessage, parseLogLineCommand, type LogLine} from '../render/log.ts';

const liveStateSelector = '#codespace-live-state';
const logViewSelector = '#codespace-log-view';
const maxRefreshBackoff = 30000;
const logRenderBatchLines = 500;

type CodespaceLogLine = {
  timestamp: number;
  message: string;
};

const openCodespaceLogGroupBodies = new WeakMap<HTMLElement, HTMLElement[]>();

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
      detail.hidden = detail.getAttribute('data-codespace-environment-detail') !== tag;
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
    if (environmentInput?.value) return;
    event.preventDefault();
    environmentDropdown?.classList.add('error');
    environmentDropdown?.setAttribute('aria-invalid', 'true');
    environmentDropdown?.focus();
  });

  const devContainerSelect = form.querySelector<HTMLSelectElement>('[data-codespace-dev-container]');
  devContainerSelect?.addEventListener('change', () => {
    const previewURL = new URL(devContainerSelect.getAttribute('data-preview-url')!, window.location.href);
    previewURL.searchParams.set('ref_type', devContainerSelect.getAttribute('data-ref-type')!);
    previewURL.searchParams.set('ref_name', devContainerSelect.getAttribute('data-ref-name')!);
    previewURL.searchParams.set('dev_container', devContainerSelect.value);
    if (environmentInput?.value) previewURL.searchParams.set('environment_tag', environmentInput.value);
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

  const logEl = document.querySelector<HTMLElement>(logViewSelector);
  if (logEl) {
    initCodespaceLogControls(logEl);
    refreshCodespaceLog(logEl, 0);
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
  for (const button of root.querySelectorAll<HTMLElement>('.codespace-settings-button:not([data-codespace-initialized])')) {
    button.setAttribute('data-codespace-initialized', 'true');
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
      form.querySelector<HTMLElement>('[data-auto-stop-out-of-range]')!.hidden = button.getAttribute('data-auto-stop-out-of-range') !== 'true';
      setAutoStopAvailability(form, button.getAttribute('data-auto-stop-configurable') === 'true');
      syncCodespaceAutoStopFields(form, false);
      showFomanticModal(form);
    });
  }
}

function setAutoStopAvailability(form: HTMLFormElement, configurable: boolean) {
  form.querySelector<HTMLFieldSetElement>('[data-auto-stop-fields]')!.disabled = !configurable;
  form.querySelector<HTMLElement>('[data-auto-stop-unavailable]')!.hidden = configurable;
  form.querySelector<HTMLButtonElement>('button[type="submit"]')!.disabled = !configurable;
}

function syncCodespaceAutoStopFields(form: HTMLFormElement, focus: boolean) {
  const customFields = form.querySelector<HTMLElement>('[data-auto-stop-custom-fields]')!;
  const timeoutInput = customFields.querySelector<HTMLInputElement>('input[name="timeout_value"]')!;
  const timeoutUnit = customFields.querySelector<HTMLSelectElement>('select[name="timeout_unit"]')!;
  const customSelected = form.querySelector<HTMLInputElement>('input[name="mode"]:checked')!.value === 'custom';
  customFields.hidden = !customSelected;
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
  setTimeout(() => {
    if (document.visibilityState === 'hidden') {
      waitForVisible(() => scheduleCodespaceStateRefresh(stateEl, failureCount));
      return;
    }
    refreshCodespaceState(stateEl, failureCount);
  }, refreshDelay(refreshAfter, failureCount));
}

async function refreshCodespaceState(stateEl: HTMLElement, failureCount: number) {
  if (!stateEl.isConnected || stateEl.getAttribute('data-refreshing') === 'true') return;
  const stateUrl = stateEl.getAttribute('data-state-url');
  if (!stateUrl) return;
  stateEl.setAttribute('data-refreshing', 'true');

  let response: Response;
  try {
    response = await GET(stateUrl);
  } catch {
    stateEl.removeAttribute('data-refreshing');
    scheduleCodespaceStateRefresh(stateEl, failureCount + 1);
    return;
  }
  if (!response.ok) {
    stateEl.removeAttribute('data-refreshing');
    scheduleCodespaceStateRefresh(stateEl, failureCount + 1);
    return;
  }

  const nextDocument = new DOMParser().parseFromString(await response.text(), 'text/html');
  const nextStateEl = nextDocument.querySelector<HTMLElement>(liveStateSelector);
  if (!nextStateEl) {
    stateEl.removeAttribute('data-refreshing');
    scheduleCodespaceStateRefresh(stateEl, failureCount + 1);
    return;
  }

  const detailPage = stateEl.closest<HTMLElement>('.codespace-detail-page');
  const currentMode = stateEl.getAttribute('data-detail-mode');
  const nextMode = nextStateEl.getAttribute('data-detail-mode');
  const explicitTab = detailPage?.getAttribute('data-codespace-tab-explicit') === 'true';
  if (!explicitTab && currentMode && nextMode && currentMode !== nextMode) {
    window.location.reload();
    return;
  }

  stateEl.replaceWith(nextStateEl);
  const settingsForm = document.querySelector<HTMLFormElement>('#codespace-settings-modal');
  if (settingsForm) initCodespaceSettingsButtons(nextStateEl, settingsForm);
  if (settingsForm?.classList.contains('visible')) {
    setAutoStopAvailability(settingsForm, nextStateEl.getAttribute('data-auto-stop-configurable') === 'true');
    syncCodespaceAutoStopFields(settingsForm, false);
  }
  scheduleCodespaceStateRefresh(nextStateEl, 0);
}

function initCodespaceLogControls(logEl: HTMLElement) {
  const panel = logEl.closest<HTMLElement>('.codespace-log-panel')!;
  const timestampButton = panel.querySelector<HTMLButtonElement>('[data-codespace-log-toggle-timestamps]')!;
  timestampButton.addEventListener('click', () => {
    const visible = logEl.classList.toggle('show-timestamps');
    timestampButton.setAttribute('aria-pressed', String(visible));
    panel.querySelector<HTMLElement>('[data-codespace-log-timestamp-check]')!.classList.toggle('tw-invisible', !visible);
  });

  const fullScreenButton = panel.querySelector<HTMLButtonElement>('[data-codespace-log-fullscreen]')!;
  fullScreenButton.addEventListener('click', () => {
    const fullScreen = !panel.classList.contains('fullscreen');
    toggleFullScreen(panel, fullScreen, '.codespace-detail-layout');
    fullScreenButton.setAttribute('aria-pressed', String(fullScreen));
    fullScreenButton.querySelector('span')!.textContent = fullScreen ? fullScreenButton.getAttribute('data-exit-label')! : fullScreenButton.getAttribute('data-enter-label')!;
  });
}

function scheduleCodespaceLogRefresh(logEl: HTMLElement, failureCount: number) {
  const refreshAfter = Number(logEl.getAttribute('data-log-refresh-after-ms'));
  if (!Number.isFinite(refreshAfter) || refreshAfter <= 0) return;
  setTimeout(() => {
    if (document.visibilityState === 'hidden') {
      waitForVisible(() => scheduleCodespaceLogRefresh(logEl, failureCount));
      return;
    }
    refreshCodespaceLog(logEl, failureCount);
  }, refreshDelay(refreshAfter, failureCount));
}

function finishCodespaceLogRefresh(logEl: HTMLElement, loadingEl: HTMLElement) {
  logEl.removeAttribute('data-refreshing');
  loadingEl.classList.add('tw-hidden');
}

async function refreshCodespaceLog(logEl: HTMLElement, failureCount: number) {
  if (!logEl.isConnected || logEl.getAttribute('data-refreshing') === 'true') return;
  const logUrl = logEl.getAttribute('data-log-url');
  const offset = logEl.getAttribute('data-log-next-offset');
  if (!logUrl || offset === null) return;
  const loadingEl = logEl.closest<HTMLElement>('.codespace-log-panel')!.querySelector<HTMLElement>('[data-codespace-log-loading]')!;
  logEl.setAttribute('data-refreshing', 'true');
  loadingEl.classList.remove('tw-hidden');

  let response: Response;
  try {
    response = await GET(`${logUrl}?offset=${encodeURIComponent(offset)}`);
  } catch {
    finishCodespaceLogRefresh(logEl, loadingEl);
    scheduleCodespaceLogRefresh(logEl, failureCount + 1);
    return;
  }
  if (!response.ok) {
    finishCodespaceLogRefresh(logEl, loadingEl);
    if (response.status === 409 && logEl.getAttribute('data-log-reset') !== 'true') {
      logEl.replaceChildren();
      logEl.setAttribute('data-log-next-offset', '0');
      logEl.setAttribute('data-log-line-count', '0');
      logEl.setAttribute('data-log-reset', 'true');
      openCodespaceLogGroupBodies.delete(logEl);
      scheduleCodespaceLogRefresh(logEl, 0);
      return;
    }
    if (response.status === 403 || response.status === 404 || response.status === 409) {
      showCodespaceLogError(logEl);
      return;
    }
    scheduleCodespaceLogRefresh(logEl, failureCount + 1);
    return;
  }

  let result: {lines?: CodespaceLogLine[]; next_offset?: number; eof?: boolean; operation_active?: boolean};
  try {
    result = await response.json();
  } catch {
    finishCodespaceLogRefresh(logEl, loadingEl);
    scheduleCodespaceLogRefresh(logEl, failureCount + 1);
    return;
  }
  if (Array.isArray(result.lines) && result.lines.length > 0) {
    const followNewLines = isLogScrolledToBottom(logEl);
    if (logEl.getAttribute('data-log-empty') === 'true') {
      logEl.querySelector('[data-log-empty-message]')?.remove();
      logEl.setAttribute('data-log-empty', 'false');
    }
    await appendCodespaceLogLines(logEl, result.lines);
    if (followNewLines) logEl.scrollTop = logEl.scrollHeight;
  }
  const currentOffset = Number(offset);
  const nextOffset = Number(result.next_offset);
  if (Number.isFinite(nextOffset)) logEl.setAttribute('data-log-next-offset', String(nextOffset));
  logEl.removeAttribute('data-log-reset');
  logEl.setAttribute('data-log-eof', String(Boolean(result.eof)));

  if (!result.eof) {
    if (Number.isFinite(currentOffset) && nextOffset > currentOffset) {
      logEl.removeAttribute('data-refreshing');
      requestAnimationFrame(() => refreshCodespaceLog(logEl, 0));
      return;
    }
    finishCodespaceLogRefresh(logEl, loadingEl);
    scheduleCodespaceLogRefresh(logEl, failureCount + 1);
    return;
  }

  finishCodespaceLogRefresh(logEl, loadingEl);
  if (result.operation_active) {
    logEl.removeAttribute('data-log-inactive-eof');
  } else if (logEl.getAttribute('data-log-inactive-eof') === String(nextOffset)) {
    return;
  } else {
    // A control-plane state change can commit before its diagnostic line is appended.
    logEl.setAttribute('data-log-inactive-eof', String(nextOffset));
  }
  scheduleCodespaceLogRefresh(logEl, 0);
}

async function appendCodespaceLogLines(logEl: HTMLElement, lines: CodespaceLogLine[]) {
  let lineNumber = Number(logEl.getAttribute('data-log-line-count')) || 0;
  let fragment = document.createDocumentFragment();
  const groupBodies = openCodespaceLogGroupBodies.get(logEl) ?? [];
  for (const [index, logLine] of lines.entries()) {
    const parsedLine: LogLine = {index: lineNumber + 1, timestamp: logLine.timestamp, message: logLine.message};
    const command = parseLogLineCommand(parsedLine);
    if (command?.name === 'group') {
      const details = document.createElement('details');
      details.className = 'codespace-log-group';
      details.open = true;
      const summary = document.createElement('summary');
      summary.className = 'codespace-log-group-summary';
      summary.textContent = decodeLogLineMessage(parsedLine, command).trim();
      const body = document.createElement('div');
      body.className = 'codespace-log-group-body';
      details.append(summary, body);
      (groupBodies.at(-1) ?? fragment).append(details);
      groupBodies.push(body);
    } else if (command?.name === 'endgroup') {
      const body = groupBodies.pop();
      const details = body?.parentElement as HTMLDetailsElement | null;
      if (details) details.open = details.getAttribute('data-log-error') === 'true';
    } else if (command?.name !== 'hidden') {
      lineNumber += 1;
      const line = document.createElement('div');
      line.className = 'codespace-log-line';
      switch (command?.name) {
        case 'error':
        case 'warning':
        case 'notice':
        case 'debug':
          line.classList.add(`codespace-log-line-${command.name}`);
      }

      const number = document.createElement('span');
      number.className = 'codespace-log-line-number';
      number.setAttribute('aria-hidden', 'true');
      number.textContent = String(lineNumber);

      const timestamp = document.createElement('time');
      timestamp.className = 'codespace-log-line-timestamp';
      timestamp.dateTime = new Date(logLine.timestamp * 1000).toISOString();
      timestamp.textContent = formatDatetime(logLine.timestamp * 1000);

      const message = createLogLineMessage({...parsedLine, index: lineNumber}, command);
      message.classList.add('codespace-log-line-message');
      if (command?.name === 'error') {
        for (const body of groupBodies) body.parentElement!.setAttribute('data-log-error', 'true');
      }

      line.append(number, timestamp, message);
      (groupBodies.at(-1) ?? fragment).append(line);
    }
    if ((index + 1) % logRenderBatchLines === 0 && index + 1 < lines.length) {
      logEl.append(fragment);
      fragment = document.createDocumentFragment();
      await new Promise<void>((resolve) => requestAnimationFrame(() => resolve()));
    }
  }
  logEl.append(fragment);
  openCodespaceLogGroupBodies.set(logEl, groupBodies);
  logEl.setAttribute('data-log-line-count', String(lineNumber));
}

function showCodespaceLogError(logEl: HTMLElement) {
  logEl.replaceChildren();
  const error = document.createElement('div');
  error.className = 'codespace-log-empty';
  error.textContent = logEl.getAttribute('data-log-error-message')!;
  logEl.append(error);
  logEl.setAttribute('data-log-empty', 'false');
  openCodespaceLogGroupBodies.delete(logEl);
}

function isLogScrolledToBottom(logEl: HTMLElement) {
  return logEl.scrollHeight - logEl.scrollTop - logEl.clientHeight <= 32;
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
