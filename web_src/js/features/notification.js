import $ from 'jquery';

const {appSubUrl, csrfToken, notificationSettings, assetVersionEncoded} = window.config;
let notificationSequenceNumber = 0;

export function initNotificationsTable() {
  const table = document.getElementById('notification_table');
  if (!table) return;

  // when page restores from bfcache, delete previously clicked items
  window.addEventListener('pageshow', (e) => {
    if (e.persisted) { // page was restored from bfcache
      const table = document.getElementById('notification_table');
      const unreadCountEl = document.querySelector('.notifications-unread-count');
      let unreadCount = parseInt(unreadCountEl.textContent);
      for (const item of table.querySelectorAll('.notifications-item[data-remove="true"]')) {
        item.remove();
        unreadCount -= 1;
      }
      unreadCountEl.textContent = unreadCount;
    }
  });

  // mark clicked unread links for deletion on bfcache restore
  for (const link of table.querySelectorAll('.notifications-item[data-status="1"] .notifications-link')) {
    link.addEventListener('click', (e) => {
      e.target.closest('.notifications-item').setAttribute('data-remove', 'true');
    });
  }

  $('#notification_table .button').on('click', function () {
    (async () => {
      const data = await updateNotification(
        $(this).data('url'),
        $(this).data('status'),
        $(this).data('page'),
        $(this).data('q'),
        $(this).data('notification-id'),
      );

      if ($(data).data('sequence-number') === notificationSequenceNumber) {
        $('#notification_div').replaceWith(data);
        initNotificationsTable();
      }
      await updateNotificationCount();
    })();
    return false;
  });
}

async function receiveUpdateCount(event) {
  try {
    const data = JSON.parse(event.data);

    for (const count of document.querySelectorAll('.notification_count')) {
      count.classList.toggle('gt-hidden', data.Count === 0);
      count.textContent = `${data.Count}`;
    }
    await updateNotificationTable();
  } catch (error) {
    console.error(error, event);
  }
}

export function initNotificationCount() {
  const notificationCount = $('.notification_count');

  if (!notificationCount.length) {
    return;
  }

  let usingPeriodicPoller = false;
  const startPeriodicPoller = (timeout, lastCount) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    usingPeriodicPoller = true;
    lastCount = lastCount ?? notificationCount.text();
    setTimeout(async () => {
      await updateNotificationCountWithCallback(startPeriodicPoller, timeout, lastCount);
    }, timeout);
  };

  if (notificationSettings.EventSourceUpdateTime > 0 && window.EventSource && window.SharedWorker) {
    // Try to connect to the event source via the shared worker first
    const worker = new SharedWorker(`${__webpack_public_path__}js/eventsource.sharedworker.js?v=${assetVersionEncoded}`, 'notification-worker');
    worker.addEventListener('error', (event) => {
      console.error('worker error', event);
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('unable to deserialize message');
    });
    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${appSubUrl}/user/events`,
    });
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }
      if (event.data.type === 'notification-count') {
        const _promise = receiveUpdateCount(event.data);
      } else if (event.data.type === 'no-event-source') {
        // browser doesn't support EventSource, falling back to periodic poller
        if (!usingPeriodicPoller) startPeriodicPoller(notificationSettings.MinTimeout);
      } else if (event.data.type === 'error') {
        console.error('worker port event error', event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') {
          return;
        }
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
        window.location.href = appSubUrl;
      } else if (event.data.type === 'close') {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }
    });
    worker.port.addEventListener('error', (e) => {
      console.error('worker port error', e);
    });
    worker.port.start();
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({
        type: 'close',
      });
      worker.port.close();
    });

    return;
  }

  startPeriodicPoller(notificationSettings.MinTimeout);
}

async function updateNotificationCountWithCallback(callback, timeout, lastCount) {
  const currentCount = $('.notification_count').text();
  if (lastCount !== currentCount) {
    callback(notificationSettings.MinTimeout, currentCount);
    return;
  }

  const newCount = await updateNotificationCount();
  let needsUpdate = false;

  if (lastCount !== newCount) {
    needsUpdate = true;
    timeout = notificationSettings.MinTimeout;
  } else if (timeout < notificationSettings.MaxTimeout) {
    timeout += notificationSettings.TimeoutStep;
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
      url: `${appSubUrl}/notifications${window.location.search}`,
      data: {
        'div-only': true,
        'sequence-number': ++notificationSequenceNumber,
      }
    });
    if ($(data).data('sequence-number') === notificationSequenceNumber) {
      notificationDiv.replaceWith(data);
      initNotificationsTable();
    }
  }
}

async function updateNotificationCount() {
  const data = await $.ajax({
    type: 'GET',
    url: `${appSubUrl}/notifications/new`,
    headers: {
      'X-Csrf-Token': csrfToken,
    },
  });

  const notificationCount = $('.notification_count');
  if (data.new === 0) {
    notificationCount.addClass('gt-hidden');
  } else {
    notificationCount.removeClass('gt-hidden');
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
      _csrf: csrfToken,
      notification_id: notificationID,
      status,
      page,
      q,
      noredirect: true,
      'sequence-number': ++notificationSequenceNumber,
    },
  });
}
