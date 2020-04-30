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

  const notificationCount = $('.notification_count');

  if (notificationCount.length > 0) {
    const fn = (timeout, lastCount) => {
      setTimeout(async () => {
        await updateNotificationCountWithCallback(fn, timeout, lastCount);
      }, timeout);
    };

    fn(NotificationSettings.MinTimeout, notificationCount.text());
  }
}

async function updateNotificationCountWithCallback(callback, timeout, lastCount) {
  const currentCount = $('.notification_count').text();
  if (lastCount !== currentCount) {
    callback(NotificationSettings.MinTimeout, currentCount);
    return;
  }

  const newCount = await updateNotificationCount();
  let needsUpdate = false;

  if (lastCount !== newCount) {
    needsUpdate = true;
    timeout = NotificationSettings.MinTimeout;
  } else if (timeout < NotificationSettings.MaxTimeout) {
    timeout += NotificationSettings.TimeoutStep;
  }

  callback(timeout, newCount);

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

async function updateNotificationCount() {
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

  notificationCount.text(`${data.new}`);

  return `${data.new}`;
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
