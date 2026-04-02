import {createTippy} from '../modules/tippy.ts';
import {GET, POST} from '../modules/fetch.ts';
import {addDelegatedEventListener, hideElem, queryElems, showElem} from '../utils/dom.ts';
import {UserEventsSharedWorker} from '../modules/worker.ts';

const {appSubUrl, notificationSettings, enableTimeTracking} = window.config;

export function initStopwatch() {
  if (!enableTimeTracking) {
    return;
  }

  const stopwatchEls = document.querySelectorAll('.active-stopwatch');
  const stopwatchPopup = document.querySelector('.active-stopwatch-popup');

  if (!stopwatchEls.length || !stopwatchPopup) {
    return;
  }

  // global stop watch (in the head_navbar), it should always work in any case either the EventSource or the PeriodicPoller is used.
  const seconds = stopwatchEls[0]?.getAttribute('data-seconds');
  if (seconds) {
    updateStopwatchTime(parseInt(seconds));
  }

  for (const stopwatchEl of stopwatchEls) {
    stopwatchEl.removeAttribute('href'); // intended for noscript mode only

    createTippy(stopwatchEl, {
      content: stopwatchPopup.cloneNode(true) as Element,
      placement: 'bottom-end',
      trigger: 'click',
      maxWidth: 'none',
      interactive: true,
      hideOnClick: true,
      theme: 'default',
      onShow(instance) {
        // Re-clone so the tooltip always reflects the latest stopwatch state,
        // even when the icon became visible via a real-time WebSocket push.
        instance.setContent(stopwatchPopup.cloneNode(true) as Element);
      },
    });
  }

  // Handle stop/cancel from the navbar popup without triggering a page reload.
  // These forms are not form-fetch-action so they won't navigate; the WebSocket
  // push (or periodic poller) updates the icon after the action completes.
  addDelegatedEventListener(document, 'submit', '.stopwatch-commit,.stopwatch-cancel', async (form: HTMLFormElement, e: SubmitEvent) => {
    e.preventDefault();
    const action = form.getAttribute('action');
    if (!action) return;
    await POST(action, {data: new FormData(form)});
  });

  // Handle start/stop/cancel from the issue sidebar without a page reload.
  // Buttons toggle between the two groups (.issue-start-buttons / .issue-stop-cancel-buttons)
  // immediately; the navbar icon is updated by the WebSocket push or periodic poller.
  addDelegatedEventListener(document, 'click', '.issue-start-time,.issue-stop-time,.issue-cancel-time', async (btn: HTMLElement, e: MouseEvent) => {
    e.preventDefault();
    const url = btn.getAttribute('data-url');
    if (!url) return;

    const startGroup = document.querySelector('.issue-start-buttons');
    const stopGroup = document.querySelector('.issue-stop-cancel-buttons');
    const isStart = btn.classList.contains('issue-start-time');

    btn.classList.add('is-loading');
    try {
      const resp = await POST(url);
      if (!resp.ok) return;
      // Toggle sidebar button groups immediately, no reload needed.
      if (isStart) {
        hideElem(startGroup);
        showElem(stopGroup);
      } else {
        hideElem(stopGroup);
        showElem(startGroup);
      }
    } finally {
      btn.classList.remove('is-loading');
    }
  });

  let usingPeriodicPoller = false;
  const startPeriodicPoller = (timeout: number) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    usingPeriodicPoller = true;
    setTimeout(() => updateStopwatchWithCallback(startPeriodicPoller, timeout), timeout);
  };

  // if the browser supports EventSource and SharedWorker, use it instead of the periodic poller
  if (notificationSettings.EventSourceUpdateTime > 0 && window.EventSource && window.SharedWorker) {
    // Try to connect to the event source via the shared worker first
    const worker = new UserEventsSharedWorker('stopwatch-worker');
    worker.addMessageEventListener((event) => {
      if (event.data.type === 'no-event-source') {
        // browser doesn't support EventSource, falling back to periodic poller
        if (!usingPeriodicPoller) startPeriodicPoller(notificationSettings.MinTimeout);
      } else if (event.data.type === 'stopwatches') {
        updateStopwatchData(JSON.parse(event.data.data));
      }
    });
    worker.startPort();
    return;
  }

  startPeriodicPoller(notificationSettings.MinTimeout);
}

async function updateStopwatchWithCallback(callback: (timeout: number) => void, timeout: number) {
  const isSet = await updateStopwatch();

  if (!isSet) {
    timeout = notificationSettings.MinTimeout;
  } else if (timeout < notificationSettings.MaxTimeout) {
    timeout += notificationSettings.TimeoutStep;
  }

  callback(timeout);
}

async function updateStopwatch() {
  const response = await GET(`${appSubUrl}/user/stopwatches`);
  if (!response.ok) {
    console.error('Failed to fetch stopwatch data');
    return false;
  }
  const data = await response.json();
  return updateStopwatchData(data);
}

function updateStopwatchData(data: any) {
  const watch = data[0];
  const btnEls = document.querySelectorAll('.active-stopwatch');
  if (!watch) {
    hideElem(btnEls);
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    document.querySelector('.stopwatch-link')?.setAttribute('href', issueUrl);
    document.querySelector('.stopwatch-commit')?.setAttribute('action', `${issueUrl}/times/stopwatch/stop`);
    document.querySelector('.stopwatch-cancel')?.setAttribute('action', `${issueUrl}/times/stopwatch/cancel`);
    const stopwatchIssue = document.querySelector('.stopwatch-issue');
    if (stopwatchIssue) stopwatchIssue.textContent = `${repo_owner_name}/${repo_name}#${issue_index}`;
    updateStopwatchTime(seconds);
    showElem(btnEls);
  }
  return Boolean(data.length);
}

// TODO: This flickers on page load, we could avoid this by making a custom element to render time periods.
function updateStopwatchTime(seconds: number) {
  const hours = seconds / 3600 || 0;
  const minutes = seconds / 60 || 0;
  const timeText = hours >= 1 ? `${Math.round(hours)}h` : `${Math.round(minutes)}m`;
  queryElems(document, '.header-stopwatch-dot', (el) => el.textContent = timeText);
}
