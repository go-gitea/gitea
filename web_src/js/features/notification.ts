import {GET} from '../modules/fetch.ts';
import {toggleElem, createElementFromHTML} from '../utils/dom.ts';
import {UserEventsSharedWorker} from '../modules/worker.ts';

const {appSubUrl, notificationSettings} = window.config;
let notificationSequenceNumber = 0;

async function receiveUpdateCount(event: MessageEvent<{type: string, data: string}>) {
  try {
    const data = JSON.parse(event.data.data);
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
    const worker = new UserEventsSharedWorker('notification-worker');
    worker.addMessageEventListener((event: MessageEvent) => {
      if (event.data.type === 'no-event-source') {
        if (!usingPeriodicPoller) startPeriodicPoller(notificationSettings.MinTimeout);
      } else if (event.data.type === 'notification-count') {
        receiveUpdateCount(event); // no await
      }
    });
    worker.startPort();
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
  const notificationDiv = document.querySelector('#notification_div');
  if (!notificationDiv) return;

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
    }
  } catch (error) {
    console.error(error);
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
