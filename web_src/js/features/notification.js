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

async function receiveUpdateCount(event) {
  try {
    const data = JSON.parse(event.data);

    const notificationCount = $('.notification_count');
    if (data.Count === 0) {
      notificationCount.addClass('hidden');
    } else {
      notificationCount.removeClass('hidden');
    }

    notificationCount.text(`${data.Count}`);
    await updateNotificationTable();
  } catch (error) {
    console.error(error, event);
  }
}

export async function initNotificationCount() {
  const notificationCount = $('.notification_count');

  if (!notificationCount.length) {
    return;
  }

  if (NotificationSettings.EventSourceUpdateTime > 0 && !!window.EventSource) {
    // Try to connect to the event source first

    if (window.SharedWorker && NotificationSettings.UseSharedWorker) {
      // const {default: Worker} = await import(/* webpackChunkName: "eventsource" */'./eventsource.sharedworker.js');
      // const worker = Worker('notification');
      const worker = new SharedWorker(`${__webpack_public_path__}js/eventsource.sharedworker.js`, 'notification-worker');
      // worker.port.addEventListener('message', (event) => {
      //   console.log(event.data);
      // }, false);
      // worker.port.start();
      worker.addEventListener('error', (event) => {
        console.error(event);
      }, false);
      worker.port.onmessageeerror = (event) => {
        console.error(event);
      };
      worker.port.postMessage({
        type: 'start',
        url: `${window.location.protocol}//${window.location.host}${AppSubUrl}/user/events`,
      });
      worker.port.addEventListener('message', (e) => {
        if (!e.data || !e.data.type) {
          console.error(e);
          return;
        }
        switch (event.data.type) {
          case 'notification-count':
            receiveUpdateCount(e.data);
            return;
          case 'error':
            console.error(e.data);
            return;
          case 'logout': {
            if (e.data !== 'here') {
              return;
            }
            worker.port.postMessage({
              type: 'close',
            });
            worker.port.close();
            window.location.href = AppSubUrl;
            return;
          }
          default:
            return;
        }
      }, false);
      worker.port.addEventListener('error', (e) => {
        console.error(e);
      }, false);
      worker.port.start();
      window.addEventListener('beforeunload', () => {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }, false);

      return;
    }

    if (window.Worker && NotificationSettings.UseWorker) {
      const {default: Worker} = await import(/* webpackChunkName: "eventsource" */'./eventsource.worker.js');
      const worker = new Worker();
      worker.postMessage({
        type: 'start',
        url: `${window.location.protocol}//${window.location.host}${AppSubUrl}/user/events`,
      });
      worker.addEventListener('notification-count', receiveUpdateCount, false);
      worker.addEventListener('logout', (e) => {
        if (e.data !== 'here') {
          return;
        }
        worker.postMessage({
          type: 'close',
        });
        worker.terminate();
        window.location.href = AppSubUrl;
      }, false);
      window.addEventListener('beforeunload', () => {
        worker.postMessage({
          type: 'close',
        });
        worker.terminate();
      }, false);
      return;
    }

    if (window.EventSource && NotificationSettings.UsePlainEventSource) {
      const eventSource = new EventSource(`${AppSubUrl}/user/events`);
      eventSource.addEventListener('notification-count', receiveUpdateCount, false);
      eventSource.addEventListener('logout', (e) => {
        if (e.data !== 'here') {
          return;
        }
        eventSource.close();
        window.location.href = AppSubUrl;
      }, false);
      window.addEventListener('beforeunload', () => {
        eventSource.close();
      }, false);
      return;
    }
  }

  if (NotificationSettings.MinTimeout <= 0) {
    return;
  }

  const fn = (timeout, lastCount) => {
    setTimeout(async () => {
      await updateNotificationCountWithCallback(fn, timeout, lastCount);
    }, timeout);
  };

  fn(NotificationSettings.MinTimeout, notificationCount.text());
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
  if (needsUpdate) {
    await updateNotificationTable();
  }
}

async function updateNotificationTable() {
  const notificationDiv = $('#notification_div');
  if (notificationDiv.length > 0) {
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
