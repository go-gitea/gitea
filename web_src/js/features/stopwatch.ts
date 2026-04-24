import {createTippy} from '../modules/tippy.ts';
import {GET} from '../modules/fetch.ts';
import {hideElem, queryElems, showElem} from '../utils/dom.ts';
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

  // Always initialise the icon + popup even when no stopwatch is currently active,
  // so cross-tab WebSocket pushes have a DOM element to toggle visibility on.
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
        // Re-clone on every open so the popup reflects the latest stopwatch state,
        // including the case where the icon became visible via a real-time push.
        instance.setContent(stopwatchPopup.cloneNode(true) as Element);
      },
    });
  }

  const startPeriodicPoller = (timeout: number) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    setTimeout(() => updateStopwatchWithCallback(startPeriodicPoller, timeout), timeout);
  };

  // if the browser supports WebSocket and SharedWorker, use push updates.
  // Fall back to periodic polling only when the worker signals that the
  // WebSocket could not be established.
  if (window.WebSocket && window.SharedWorker) {
    let pollerStarted = false;
    const worker = new UserEventsSharedWorker('stopwatch-worker');
    worker.addMessageEventListener((event) => {
      if (event.data.type === 'stopwatches') {
        updateStopwatchData(JSON.parse(event.data.data));
      } else if (event.data.type === 'push-unavailable' && !pollerStarted) {
        pollerStarted = true;
        startPeriodicPoller(notificationSettings.MinTimeout);
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
