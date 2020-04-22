const {AppSubUrl, csrf} = window.config;

export function initNotificationsTable() {
  $('#notification_table .button').click(function () {
    updateNotification(
      $(this).data('url'),
      $(this).data('status'),
      $(this).data('page'),
      $(this).data('q'),
      $(this).data('notification-id')
    ).then((data) => {
      $('#notification_div').replaceWith(data);
      initNotificationsTable();
      updateNotificationCount();
    });
    return false;
  });
}

export function initNotificationCount() {
  if ($('.notification_count').length > 0) {
    const lastCount = $('.notification_count').text();
    const fn = (callback, timeout, lastCount) => {
      setTimeout(() => {
        updateNotificationCount(callback, timeout, lastCount);
      }, timeout);
    };

    fn(fn, 10000, lastCount);
  }
}

function updateNotificationCount(callback, timeout, lastCount) {
  const currentCount = $('.notification_count').text();
  if (callback && (lastCount !== currentCount)) {
    callback(callback, 10000, currentCount);
    return;
  }
  $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/notifications/new`,
    data: {
      _csrf: csrf,
    },
  }).then((data) => {
    const notificationCount = $('.notification_count');
    const notificationDependent = $('.notification_dependent');
    if (data.new === 0) {
      notificationCount.addClass('hidden');
      notificationDependent.addClass('hide');
    } else {
      notificationCount.removeClass('hidden');
      notificationDependent.removeClass('hide');
    }
    const currentCount = $('.notification_count').text();
    if (lastCount !== `${data.new}` || currentCount !== `${data.new}`) {
      notificationCount.text(data.new);
      timeout = 10000;
    } else if (timeout < 60000) {
      timeout += 10000;
    }
    return {
      timeout,
      nextCount: `${data.new}`,
    };
  }).then((data) => {
    if (callback) {
      callback(callback, data.timeout, data.nextCount);
    }
  });
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
