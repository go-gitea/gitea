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
  setInterval(() => {
    updateNotificationCount();
  }, 5000);
}

function updateNotificationCount() {
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
      notificationCount.text(data.new);
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
