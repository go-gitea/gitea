import {GET} from '../modules/fetch.ts';
import {toggleElem, type DOMEvent, createElementFromHTML} from '../utils/dom.ts';
import {logoutFromWorker} from '../modules/worker.ts';

const {appSubUrl, notificationSettings, assetVersionEncoded} = window.config;
let notificationSequenceNumber = 0;

export function initNotificationsTable() {
  const table = document.querySelector('#notification_table');
  if (!table) return;

  // when page restores from bfcache, delete previously clicked items
  window.addEventListener('pageshow', (e) => {
    if (e.persisted) { // page was restored from bfcache
      const table = document.querySelector('#notification_table');
      const unreadCountEl = document.querySelector<HTMLElement>('.notifications-unread-count');
      let unreadCount = parseInt(unreadCountEl.textContent);
      for (const item of table.querySelectorAll('.notifications-item[data-remove="true"]')) {
        item.remove();
        unreadCount -= 1;
      }
      unreadCountEl.textContent = String(unreadCount);
    }
  });

  // mark clicked unread links for deletion on bfcache restore
  for (const link of table.querySelectorAll<HTMLAnchorElement>('.notifications-item[data-status="1"] .notifications-link')) {
    link.addEventListener('click', (e: DOMEvent<MouseEvent>) => {
      e.target.closest('.notifications-item').setAttribute('data-remove', 'true');
    });
  }
}

async function receiveUpdateCount(event: MessageEvent) {
  try {
    const data = JSON.parse(event.data);

    for (const count of document.querySelectorAll('.notification_count')) {
      count.classList.toggle('tw-hidden', data.Count === 0);
      count.textContent = `${data.Count}`;
    }
    await updateNotificationTable();
  } catch (error) {
    console.error(error, event);
  }
}

export function initNotificationCount() {
  if (!document.querySelector('.notification_count')) return;

  let usingPeriodicPoller = false;
  const startPeriodicPoller = (timeout: number, lastCount?: number) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    usingPeriodicPoller = true;
    lastCount = lastCount ?? getCurrentCount();
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
    worker.port.addEventListener('message', (event: MessageEvent) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }
      if (event.data.type === 'notification-count') {
        receiveUpdateCount(event); // no await
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
        logoutFromWorker();
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

function getCurrentCount() {
  return Number(document.querySelector('.notification_count').textContent ?? '0');
}

async function updateNotificationCountWithCallback(callback: (timeout: number, newCount: number) => void, timeout: number, lastCount: number) {
  const currentCount = getCurrentCount();
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
  const notificationDiv = document.querySelector('#notification_div');
  if (notificationDiv) {
    try {
      const params = new URLSearchParams(window.location.search);
      params.set('div-only', String(true));
      params.set('sequence-number', String(++notificationSequenceNumber));
      const response = await GET(`${appSubUrl}/notifications?${params.toString()}`);

      if (!response.ok) {
        throw new Error('Failed to fetch notification table');
      }

      const data = await response.text();
      const el = createElementFromHTML(data);
      if (parseInt(el.getAttribute('data-sequence-number')) === notificationSequenceNumber) {
        notificationDiv.outerHTML = data;
        initNotificationsTable();
      }
    } catch (error) {
      console.error(error);
    }
  }
}

async function updateNotificationCount(): Promise<number> {
  try {
    const response = await GET(`${appSubUrl}/notifications/new`);

    if (!response.ok) {
      throw new Error('Failed to fetch notification count');
    }

    const data = await response.json();

    toggleElem('.notification_count', data.new !== 0);

    for (const el of document.querySelectorAll('.notification_count')) {
      el.textContent = `${data.new}`;
    }

    return data.new as number;
  } catch (error) {
    console.error(error);
    return 0;
  }
}
