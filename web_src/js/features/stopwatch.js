import prettyMilliseconds from 'pretty-ms';
const {AppSubUrl, csrf, NotificationSettings} = window.config;

let updateTimeInterval = null; // holds setInterval id when active

export async function initStopwatch() {
  const stopwatchEl = $('.active-stopwatch-trigger');

  stopwatchEl.removeAttr('href'); // intended for noscript mode only
  stopwatchEl.popup({
    position: 'bottom right',
    hoverable: true,
  });

  // form handlers
  $('form > button', stopwatchEl).on('click', function () {
    $(this).parent().trigger('submit');
  });

  if (!stopwatchEl || NotificationSettings.MinTimeout <= 0) {
    return;
  }

  const fn = (timeout) => {
    setTimeout(async () => {
      await updateStopwatchWithCallback(fn, timeout);
    }, timeout);
  };

  fn(NotificationSettings.MinTimeout);

  const currSeconds = $('.stopwatch-time').data('seconds');
  if (currSeconds) {
    updateTimeInterval = updateStopwatchTime(currSeconds);
  }
}

async function updateStopwatchWithCallback(callback, timeout) {
  const isSet = await updateStopwatch();

  if (!isSet) {
    timeout = NotificationSettings.MinTimeout;
  } else if (timeout < NotificationSettings.MaxTimeout) {
    timeout += NotificationSettings.TimeoutStep;
  }

  callback(timeout);
}

async function updateStopwatch() {
  const data = await $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/user/stopwatches`,
    headers: {'X-Csrf-Token': csrf},
  });

  if (updateTimeInterval) {
    clearInterval(updateTimeInterval);
    updateTimeInterval = null;
  }

  const watch = data[0];
  const btnEl = $('.active-stopwatch-trigger');
  if (!watch) {
    btnEl.addClass('hidden');
  } else {
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${AppSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    $('.stopwatch-link').attr('href', issueUrl);
    $('.stopwatch-commit').attr('action', `${issueUrl}/times/stopwatch/toggle`);
    $('.stopwatch-cancel').attr('action', `${issueUrl}/times/stopwatch/cancel`);
    $('.stopwatch-issue').text(`${repo_owner_name}/${repo_name}#${issue_index}`);
    $('.stopwatch-time').text(prettyMilliseconds(seconds * 1000));
    updateStopwatchTime(seconds);
    btnEl.removeClass('hidden');
  }

  return !!data.length;
}

async function updateStopwatchTime(seconds) {
  const secs = parseInt(seconds);
  if (!Number.isFinite(secs)) return;

  const start = Date.now();
  updateTimeInterval = setInterval(() => {
    const delta = Date.now() - start;
    const dur = prettyMilliseconds(secs * 1000 + delta, {compact: true});
    $('.stopwatch-time').text(dur);
  }, 1000);
}
