import {GET} from '../modules/fetch.ts';
import {toggleElem, createElementFromHTML} from '../utils/dom.ts';
import {logoutFromWorker} from '../modules/worker.ts';

const {appSubUrl, notificationSettings, assetVersionEncoded} = window.config;
let notificationSequenceNumber = 0;

async function receiveUpdateCount(event: MessageEvent<{type: string, count: number}>) {
  try {
    const {count} = event.data;
    for (const el of document.querySelectorAll('.notification_count')) {
      el.classList.toggle('tw-hidden', count === 0);
      el.textContent = `${count}`;
    }
    await updateNotificationTable();
  } catch (error) {
    console.error(error, event);
  }
}

export function initNotificationCount() {
  if (!document.querySelector('.notification_count')) return;

  const startPeriodicPoller = (timeout: number, lastCount?: number) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    lastCount = lastCount ?? getCurrentCount();
    setTimeout(async () => {
      await updateNotificationCountWithCallback(startPeriodicPoller, timeout, lastCount);
    }, timeout);
  };

  if (notificationSettings.EventSourceUpdateTime > 0 && window.SharedWorker) {
    // Connect via WebSocket SharedWorker (one connection shared across all tabs)
    const wsUrl = `${window.location.origin}${appSubUrl}/-/ws`.replace(/^http/, 'ws');
    const worker = new SharedWorker(`${window.__webpack_public_path__}js/websocket.sharedworker.js?v=${assetVersionEncoded}`, 'notification-worker-ws');
    worker.addEventListener('error', (event) => {
      console.error('worker error', event);
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('unable to deserialize message');
    });
    worker.port.postMessage({type: 'start', url: wsUrl});
    worker.port.addEventListener('message', (event: MessageEvent<{type: string, count: number, message?: string}>) => {
      if (!event.data || !event.data.type) {
        console.error('unknown worker message event', event);
        return;
      }
      if (event.data.type === 'notification-count') {
        receiveUpdateCount(event); // no await
      } else if (event.data.type === 'error') {
        console.error('worker port event error', event.data);
      } else if (event.data.type === 'logout') {
        worker.port.postMessage({type: 'close'});
        worker.port.close();
        logoutFromWorker();
      }
    });
    worker.port.addEventListener('error', (e) => {
      console.error('worker port error', e);
    });
    worker.port.start();
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({type: 'close'});
      worker.port.close();
    });

    return;
  }

  startPeriodicPoller(notificationSettings.MinTimeout);
}

function getCurrentCount() {
  return Number(document.querySelector('.notification_count')!.textContent ?? '0');
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
  let notificationDiv = document.querySelector('#notification_div');
  if (notificationDiv) {
    try {
      const params = new URLSearchParams(window.location.search);
      params.set('div-only', 'true');
      params.set('sequence-number', String(++notificationSequenceNumber));
      const response = await GET(`${appSubUrl}/notifications?${params.toString()}`);

      if (!response.ok) {
        throw new Error('Failed to fetch notification table');
      }

      const data = await response.text();
      const el = createElementFromHTML(data);
      if (parseInt(el.getAttribute('data-sequence-number')!) === notificationSequenceNumber) {
        notificationDiv.outerHTML = data;
        notificationDiv = document.querySelector('#notification_div')!;
        window.htmx.process(notificationDiv); // when using htmx, we must always remember to process the new content changed by us
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
