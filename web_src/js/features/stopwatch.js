const {AppSubUrl, csrf, NotificationSettings} = window.config;

export async function initStopwatch() {
  const stopwatchEl = $('.active-stopwatch-trigger');

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
  const btnEl = $('.active-stopwatch-trigger');
  if (!watch) {
    btnEl.addClass('hidden');
  } else {
    const {repo_owner_name, repo_name, issue_index, duration} = watch;
    const issueUrl = `${AppSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    $('.stopwatch-link').attr('href', issueUrl);
    $('.stopwatch-commit').attr('action', `${issueUrl}/times/stopwatch/toggle`);
    $('.stopwatch-cancel').attr('action', `${issueUrl}/times/stopwatch/cancel`);
    $('.stopwatch-issue').text(`${repo_owner_name}/${repo_name}#${issue_index}`);
    $('.stopwatch-time').text(`${duration}`);
    btnEl.removeClass('hidden');
  }

  return !!data.length;
}
