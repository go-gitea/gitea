const {AppSubUrl, csrf, NotificationSettings} = window.config;

export async function initStopwatch() {
  const stopwatchEl = $('.active-stopwatch');
  if (!stopwatchEl || NotificationSettings.MinTimeout <= 0) {
    return;
  }

  const fn = (timeout) => {
    setTimeout(async () => {
      await updateStopwatchWithCallback(fn, timeout);
    }, timeout);
  };

  fn(NotificationSettings.MinTimeout);
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

  const watch = data[0];
  const btnEl = $('.active-stopwatch');
  if (!watch) {
    btnEl.addClass('hidden');
  } else {
    $('.stopwatch-time').text(`${watch.duration}`);
    btnEl.attr('href', `${AppSubUrl}/${watch.repo_owner_name}/${watch.repo_name}/issues/${watch.issue_index}`);
    btnEl.removeClass('hidden');
  }

  return !!data.length;
}
