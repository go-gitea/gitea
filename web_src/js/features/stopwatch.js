import $ from 'jquery';
import prettyMilliseconds from 'pretty-ms';
import getMemoizedSharedWorker from './shared-worker.js';

const {appSubUrl, csrfToken, notificationSettings, enableTimeTracking} = window.config;
let updateTimeInterval = null; // holds setInterval id when active

export function initStopwatch() {
  if (!enableTimeTracking) {
    return;
  }

  const stopwatchEl = $('.active-stopwatch-trigger');

  if (!stopwatchEl.length) {
    return;
  }

  stopwatchEl.removeAttr('href'); // intended for noscript mode only
  stopwatchEl.popup({
    position: 'bottom right',
    hoverable: true,
  });

  // form handlers
  $('form > button', stopwatchEl).on('click', function () {
    $(this).parent().trigger('submit');
  });

  let worker;

  if (notificationSettings.EventSourceUpdateTime > 0 && (worker = getMemoizedSharedWorker())) {
    // Try to connect to the event source via the shared worker first
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        return;
      }
      if (event.data.type === 'stopwatches') {
        updateStopwatchData(JSON.parse(event.data.data));
      }
    });

    return;
  }

  if (notificationSettings.MinTimeout <= 0) {
    return;
  }

  const fn = (timeout) => {
    setTimeout(() => {
      const _promise = updateStopwatchWithCallback(fn, timeout);
    }, timeout);
  };

  fn(notificationSettings.MinTimeout);

  const currSeconds = $('.stopwatch-time').data('seconds');
  if (currSeconds) {
    updateTimeInterval = updateStopwatchTime(currSeconds);
  }
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
  const data = await $.ajax({
    type: 'GET',
    url: `${appSubUrl}/user/stopwatches`,
    headers: {'X-Csrf-Token': csrfToken},
  });

  if (updateTimeInterval) {
    clearInterval(updateTimeInterval);
    updateTimeInterval = null;
  }

  return updateStopwatchData(data);
}

function updateStopwatchData(data) {
  const watch = data[0];
  const btnEl = $('.active-stopwatch-trigger');
  if (!watch) {
    if (updateTimeInterval) {
      clearInterval(updateTimeInterval);
      updateTimeInterval = null;
    }
    btnEl.addClass('hidden');
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    $('.stopwatch-link').attr('href', issueUrl);
    $('.stopwatch-commit').attr('action', `${issueUrl}/times/stopwatch/toggle`);
    $('.stopwatch-cancel').attr('action', `${issueUrl}/times/stopwatch/cancel`);
    $('.stopwatch-issue').text(`${repo_owner_name}/${repo_name}#${issue_index}`);
    $('.stopwatch-time').text(prettyMilliseconds(seconds * 1000));
    updateTimeInterval = updateStopwatchTime(seconds);
    btnEl.removeClass('hidden');
  }

  return !!data.length;
}

function updateStopwatchTime(seconds) {
  const secs = parseInt(seconds);
  if (!Number.isFinite(secs)) return null;

  const start = Date.now();
  return setInterval(() => {
    const delta = Date.now() - start;
    const dur = prettyMilliseconds(secs * 1000 + delta, {compact: true});
    $('.stopwatch-time').text(dur);
  }, 1000);
}
