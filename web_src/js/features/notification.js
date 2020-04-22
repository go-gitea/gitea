const {AppSubUrl, csrf} = window.config;

export async function initNotificationsTable() {
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

export async function initNotificationCount() {
  if ($('.notification_count').length > 0) {
    const fn = (callback, timeout) => {
      setTimeout(() => {
        updateNotificationCount(callback, timeout);
      }, timeout);
    };

    fn(fn, 10000);
  }
}

function updateNotificationCount(callback, timeout) {
  $.ajax({
    type: 'GET',
    url: `${AppSubUrl}/api/v1/notifications/new`,
    data: {
      _csrf: csrf,
    },
    success: (data) => {
      const notificationCount = $('.notification_count');
      const notificationDependent = $('.notification_dependent');
      if (data.new === 0) {
        notificationCount.addClass('hidden');
        notificationDependent.addClass('hide');
      } else {
        notificationCount.removeClass('hidden');
        notificationDependent.removeClass('hide');
      }
      const currentCount = notificationCount.text();
      if (currentCount !== `${data.new}`) {
        notificationCount.text(data.new);
        timeout = 10000;
      } else if (timeout < 60000) {
        timeout += 10000;
      }
      if (callback) {
        callback(callback, timeout);
      }
    },
  });
}

function updateNotification(url, status, page, q, notificationID) {
  return new Promise(((resolve) => {
    if (status !== 'pinned') {
      $(`#notification_${notificationID}`).remove();
    }
    $.ajax({
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
      success: resolve
    });
  }));
}
