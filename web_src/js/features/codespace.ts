import {GET} from '../modules/fetch.ts';

const liveStateSelector = '#codespace-live-state';

export function initCodespaceLiveState() {
  const stateEl = document.querySelector<HTMLElement>(liveStateSelector);
  if (!stateEl) return;
  scheduleCodespaceStateRefresh(stateEl);
}

function scheduleCodespaceStateRefresh(stateEl: HTMLElement) {
  const refreshAfter = Number(stateEl.getAttribute('data-refresh-after-ms'));
  if (!Number.isFinite(refreshAfter) || refreshAfter <= 0) return;
  setTimeout(() => refreshCodespaceState(stateEl), refreshAfter);
}

async function refreshCodespaceState(stateEl: HTMLElement) {
  if (!stateEl.isConnected) return;

  const stateUrl = stateEl.getAttribute('data-state-url');
  if (!stateUrl) return;

  let response: Response;
  try {
    response = await GET(stateUrl);
  } catch {
    scheduleCodespaceStateRefresh(stateEl);
    return;
  }
  if (!response.ok) {
    scheduleCodespaceStateRefresh(stateEl);
    return;
  }

  const nextDocument = new DOMParser().parseFromString(await response.text(), 'text/html');
  const nextStateEl = nextDocument.querySelector<HTMLElement>(liveStateSelector);
  if (!nextStateEl) {
    scheduleCodespaceStateRefresh(stateEl);
    return;
  }

  stateEl.replaceWith(nextStateEl);
  scheduleCodespaceStateRefresh(nextStateEl);
}
