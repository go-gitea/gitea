import {createTippy} from '../modules/tippy.ts';
import {GET} from '../modules/fetch.ts';
import {hideElem, showElem} from '../utils/dom.ts';
import {logoutFromWorker} from '../modules/worker.ts';

const {appSubUrl, notificationSettings, enableTimeTracking, assetVersionEncoded} = window.config;

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
    });
  }

  let usingPeriodicPoller = false;
  const startPeriodicPoller = (timeout) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    usingPeriodicPoller = true;
    setTimeout(() => updateStopwatchWithCallback(startPeriodicPoller, timeout), timeout);
  };

  // if the browser supports EventSource and SharedWorker, use it instead of the periodic poller
  if (notificationSettings.EventSourceUpdateTime > 0 && window.EventSource && window.SharedWorker) {
    // Try to connect to the event source via the shared worker first
    const worker = new SharedWorker(`${__webpack_public_path__}js/eventsource.sharedworker.js?v=${assetVersionEncoded}`, 'notification-worker');
    worker.addEventListener('error', (event) => {
      console.error('worker error', event);
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('unable to deserialize message');
    });
    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${appSubUrl}/user/events`,
    });
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }
      if (event.data.type === 'stopwatches') {
        updateStopwatchData(JSON.parse(event.data.data));
      } else if (event.data.type === 'no-event-source') {
        // browser doesn't support EventSource, falling back to periodic poller
        if (!usingPeriodicPoller) startPeriodicPoller(notificationSettings.MinTimeout);
      } else if (event.data.type === 'error') {
        console.error('worker port event error', event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') {
          return;
        }
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
        logoutFromWorker();
      } else if (event.data.type === 'close') {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }
    });
    worker.port.addEventListener('error', (e) => {
      console.error('worker port error', e);
    });
    worker.port.start();
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({
        type: 'close',
      });
      worker.port.close();
    });

    return;
  }

  startPeriodicPoller(notificationSettings.MinTimeout);
}

async function updateStopwatchWithCallback(callback, timeout) {
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

function updateStopwatchData(data) {
  const watch = data[0];
  const btnEls = document.querySelectorAll('.active-stopwatch');
  if (!watch) {
    hideElem(btnEls);
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    document.querySelector('.stopwatch-link')?.setAttribute('href', issueUrl);
    document.querySelector('.stopwatch-commit')?.setAttribute('action', `${issueUrl}/times/stopwatch/toggle`);
    document.querySelector('.stopwatch-cancel')?.setAttribute('action', `${issueUrl}/times/stopwatch/cancel`);
    const stopwatchIssue = document.querySelector('.stopwatch-issue');
    if (stopwatchIssue) stopwatchIssue.textContent = `${repo_owner_name}/${repo_name}#${issue_index}`;
    updateStopwatchTime(seconds);
    showElem(btnEls);
  }
  return Boolean(data.length);
}

// TODO: This flickers on page load, we could avoid this by making a custom
// element to render time periods. Feeding a datetime in backend does not work
// when time zone between server and client differs.
function updateStopwatchTime(seconds) {
  if (!Number.isFinite(seconds)) return;
  const datetime = (new Date(Date.now() - seconds * 1000)).toISOString();
  for (const parent of document.querySelectorAll('.header-stopwatch-dot')) {
    const existing = parent.querySelector(':scope > relative-time');
    if (existing) {
      existing.setAttribute('datetime', datetime);
    } else {
      const el = document.createElement('relative-time');
      el.setAttribute('format', 'micro');
      el.setAttribute('datetime', datetime);
      el.setAttribute('lang', 'en-US');
      el.setAttribute('title', ''); // make <relative-time> show no title and therefor no tooltip
      parent.append(el);
    }
  }
}
