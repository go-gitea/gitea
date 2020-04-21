const {csrf} = window.config;

export default async function initNotificationsTable() {
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
    });
    return false;
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
