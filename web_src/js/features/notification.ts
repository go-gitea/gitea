import {GET} from '../modules/fetch.ts';
import {toggleElem, createElementFromHTML} from '../utils/dom.ts';
import {onUserEvent} from '../modules/worker.ts';

const {appSubUrl, notificationSettings} = window.config;
let notificationSequenceNumber = 0;

async function receiveUpdateCount(count: number) {
  for (const el of document.querySelectorAll('.notification_count')) {
    el.classList.toggle('tw-hidden', count === 0);
    el.textContent = `${count}`;
  }
  await updateNotificationTable();
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

  let pollerStarted = false;
  onUserEvent('notification-count', (msg) => { receiveUpdateCount(msg.count) }); // no await
  onUserEvent('push-unavailable', () => {
    if (pollerStarted) return;
    pollerStarted = true;
    startPeriodicPoller(notificationSettings.MinTimeout);
  });
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
