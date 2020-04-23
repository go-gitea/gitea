const {AppSubUrl, csrf, NotificationSettings} = window.config;

export function initNotificationsTable() {
  $('#notification_table .button').on('click', async function () {
    const data = await updateNotification(
      $(this).data('url'),
      $(this).data('status'),
      $(this).data('page'),
      $(this).data('q'),
      $(this).data('notification-id'),
    );

    $('#notification_div').replaceWith(data);
    initNotificationsTable();
    await updateNotificationCount();

    return false;
  });
}

export function initNotificationCount() {
  if (NotificationSettings.MinTimeout <= 0) {
    return;
  }

  if ($('.notification_count').length > 0) {
    const lastCount = $('.notification_count').text();
    const fn = (callback, timeout, lastCount) => {
      setTimeout(async () => {
        await updateNotificationCount(callback, timeout, lastCount);
      }, timeout);
    };

    fn(fn, NotificationSettings.MinTimeout, lastCount);
  }
}

async function updateNotificationCount(callback, timeout, lastCount) {
  let currentCount = $('.notification_count').text();
  if (callback && (lastCount !== currentCount)) {
    callback(callback, NotificationSettings.MinTimeout, currentCount);
    return;
  }

  const data = await $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/notifications/new`,
    headers: {
      'X-Csrf-Token': csrf,
    },
  });

  const notificationCount = $('.notification_count');
  if (data.new === 0) {
    notificationCount.addClass('hidden');
  } else {
    notificationCount.removeClass('hidden');
  }

  let needsUpdate = false;
  currentCount = $('.notification_count').text();
  if (lastCount !== `${data.new}` || currentCount !== `${data.new}`) {
    notificationCount.text(data.new);
    needsUpdate = true;
    timeout = NotificationSettings.MinTimeout;
  } else if (timeout < NotificationSettings.MaxTimeout) {
    timeout += NotificationSettings.TimeoutStep;
  }

  if (callback) {
    callback(callback, timeout, `${data.new}`);
    const notificationDiv = $('#notification_div');
    if (notificationDiv.length > 0 && needsUpdate) {
      const data = await $.ajax({
        type: 'GET',
        url: `${AppSubUrl}/notifications?${notificationDiv.data('params')}`,
        data: {
          'div-only': true,
        }
      });
      notificationDiv.replaceWith(data);
      initNotificationsTable();
    }
  }
}

async function updateNotification(url, status, page, q, notificationID) {
  if (status !== 'pinned') {
    $(`#notification_${notificationID}`).remove();
  }

  return $.ajax({
    type: 'POST',
    url,
    data: {
      _csrf: csrf,
      notification_id: notificationID,
      status,
      page,
      q,
      noredirect: true,
    },
  });
}
