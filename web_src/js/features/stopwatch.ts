import {createTippy} from '../modules/tippy.ts';
import {GET} from '../modules/fetch.ts';
import {hideElem, showElem} from '../utils/dom.ts';
import {onUserEvent} from '../modules/worker.ts';
import type {StopwatchData} from '../types.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';

const {appSubUrl, notificationSettings} = window.config;

export const initStopwatch = () => registerGlobalInitFunc('initActiveStopwatchNotification', (btn: HTMLElement) => {
  // Init the icon + popup even when no stopwatch is active so a real-time push has a target to toggle.
  const popup = btn.nextElementSibling!;
  const tippy = createTippy(btn, {
    content: popup,
    placement: 'bottom-end',
    trigger: 'click',
    maxWidth: 'none',
    interactive: true,
    hideOnClick: true,
    theme: 'default',
  });

  // TODO: This flickers on page load, we could avoid this by making a custom element to render time periods.
  const updateStopwatchTime = (seconds: number) => {
    const hours = seconds / 3600 || 0;
    const minutes = seconds / 60 || 0;
    btn.querySelector('.header-stopwatch-dot')!.textContent = hours >= 1 ? `${Math.round(hours)}h` : `${Math.round(minutes)}m`;
  };

  const updateStopwatchData = (data: Array<StopwatchData>) => {
    const watch = data[0];
    if (!watch) {
      tippy.hide();
      hideElem(btn);
      return false;
    }
    const {repo_owner_name, repo_name, issue_index, seconds} = watch;
    const issueUrl = `${appSubUrl}/${repo_owner_name}/${repo_name}/issues/${issue_index}`;
    popup.querySelector('.stopwatch-link')!.setAttribute('href', issueUrl);
    popup.querySelector('.stopwatch-commit')!.setAttribute('action', `${issueUrl}/times/stopwatch/stop`);
    popup.querySelector('.stopwatch-cancel')!.setAttribute('action', `${issueUrl}/times/stopwatch/cancel`);
    popup.querySelector('.stopwatch-issue')!.textContent = `${repo_owner_name}/${repo_name}#${issue_index}`;
    updateStopwatchTime(seconds);
    showElem(btn);
    return true;
  };

  const updateStopwatch = async () => {
    try {
      const response = await GET(`${appSubUrl}/user/stopwatches`);
      if (!response.ok) {
        console.error('Failed to fetch stopwatch data');
        return false;
      }
      return updateStopwatchData(await response.json());
    } catch (error) {
      console.error(error);
      return false;
    }
  };

  const startPeriodicPoller = (timeout: number) => {
    if (timeout <= 0 || !Number.isFinite(timeout)) return;
    setTimeout(async () => {
      if (!await updateStopwatch()) {
        timeout = notificationSettings.MinTimeout;
      } else if (timeout < notificationSettings.MaxTimeout) {
        timeout += notificationSettings.TimeoutStep;
      }
      startPeriodicPoller(timeout);
    }, timeout);
  };

  const seconds = btn.getAttribute('data-seconds');
  if (seconds) updateStopwatchTime(parseInt(seconds));

  let pollerStarted = false;
  onUserEvent('stopwatches', (msg) => updateStopwatchData(msg.eventData));
  // On each (re)connect, reconcile stopwatch state from the server to recover any push dropped during the connect gap.
  onUserEvent('worker-connected', () => { updateStopwatch() }); // no await
  onUserEvent('worker-unavailable', () => {
    if (pollerStarted) return;
    pollerStarted = true;
    startPeriodicPoller(notificationSettings.MinTimeout);
  });
});
