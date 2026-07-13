import {createTippy} from '../modules/tippy.ts';
import {GET} from '../modules/fetch.ts';
import {hideElem, queryElems, showElem} from '../utils/dom.ts';
import {onUserEvent} from '../modules/worker.ts';
import type {StopwatchData} from '../types.ts';

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

  // Init the icon + popup even when no stopwatch is active so a real-time push has a target to toggle.
  const seconds = stopwatchEls[0]?.getAttribute('data-seconds');
  if (seconds) {
    updateStopwatchTime(parseInt(seconds));
  }

  for (const stopwatchEl of stopwatchEls) {
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

  let pollerStarted = false;
  onUserEvent('stopwatches', (msg) => updateStopwatchData(msg.data));
  // On each (re)connect, reconcile stopwatch state from the server to recover any push dropped during the connect gap.
  onUserEvent('ws-connected', () => { updateStopwatch() }); // no await
  onUserEvent('push-unavailable', () => {
    if (pollerStarted) return;
    pollerStarted = true;
    startPeriodicPoller(notificationSettings.MinTimeout);
  });
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

function updateStopwatchData(data: Array<StopwatchData>) {
  const watch = data[0];
  const btnEls = document.querySelectorAll('.active-stopwatch');
  if (!watch) {
    hideElem(btnEls);
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    document.querySelector('.stopwatch-link')?.setAttribute('href', issueUrl);
    const commitForm = document.querySelector('.stopwatch-commit');
    if (commitForm) {
      commitForm.setAttribute('action', `${issueUrl}/times/stopwatch/stop`);
      commitForm.classList.add('form-fetch-action');
    }
    const cancelForm = document.querySelector('.stopwatch-cancel');
    if (cancelForm) {
      cancelForm.setAttribute('action', `${issueUrl}/times/stopwatch/cancel`);
      cancelForm.classList.add('form-fetch-action');
    }
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
