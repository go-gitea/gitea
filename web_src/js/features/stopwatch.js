import prettyMilliseconds from 'pretty-ms';
import {createTippy} from '../modules/tippy.js';
import {GET} from '../modules/fetch.js';
import {hideElem, showElem} from '../utils/dom.js';

const {appSubUrl, notificationSettings, enableTimeTracking, assetVersionEncoded} = window.config;

export function initStopwatch() {
  if (!enableTimeTracking) {
    return;
  }

  const stopwatchEl = document.querySelector('.active-stopwatch-trigger');
  const stopwatchPopup = document.querySelector('.active-stopwatch-popup');

  if (!stopwatchEl || !stopwatchPopup) {
    return;
  }

  stopwatchEl.removeAttribute('href'); // intended for noscript mode only

  createTippy(stopwatchEl, {
    content: stopwatchPopup,
    placement: 'bottom-end',
    trigger: 'click',
    maxWidth: 'none',
    interactive: true,
    hideOnClick: true,
  });

  // global stop watch (in the head_navbar), it should always work in any case either the EventSource or the PeriodicPoller is used.
  const currSeconds = document.querySelector('.stopwatch-time')?.getAttribute('data-seconds');
  if (currSeconds) {
    updateStopwatchTime(currSeconds);
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
        window.location.href = appSubUrl;
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
  const btnEl = document.querySelector('.active-stopwatch-trigger');
  if (!watch) {
    clearStopwatchTimer();
    hideElem(btnEl);
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    document.querySelector('.stopwatch-link')?.setAttribute('href', issueUrl);
    document.querySelector('.stopwatch-commit')?.setAttribute('action', `${issueUrl}/times/stopwatch/toggle`);
    document.querySelector('.stopwatch-cancel')?.setAttribute('action', `${issueUrl}/times/stopwatch/cancel`);
    const stopwatchIssue = document.querySelector('.stopwatch-issue');
    if (stopwatchIssue) stopwatchIssue.textContent = `${repo_owner_name}/${repo_name}#${issue_index}`;
    updateStopwatchTime(seconds);
    showElem(btnEl);
  }
  return Boolean(data.length);
}

let updateTimeIntervalId = null; // holds setInterval id when active
function clearStopwatchTimer() {
  if (updateTimeIntervalId !== null) {
    clearInterval(updateTimeIntervalId);
    updateTimeIntervalId = null;
  }
}
function updateStopwatchTime(seconds) {
  const secs = parseInt(seconds);
  if (!Number.isFinite(secs)) return;

  clearStopwatchTimer();
  const stopwatch = document.querySelector('.stopwatch-time');
  // TODO: replace with <relative-time> similar to how system status up time is shown
  const start = Date.now();
  const updateUi = () => {
    const delta = Date.now() - start;
    const dur = prettyMilliseconds(secs * 1000 + delta, {compact: true});
    if (stopwatch) stopwatch.textContent = dur;
  };
  updateUi();
  updateTimeIntervalId = setInterval(updateUi, 1000);
}
