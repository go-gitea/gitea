import {parseIssuePageInfo} from '../utils.ts';

const {appSubUrl, notificationSettings, assetVersionEncoded} = window.config;

const eventMessages: Record<string, string> = {
  'comment': 'A new comment was posted.',
  'issue-opened': 'A new issue was opened.',
  'issue-closed': 'This issue was closed.',
  'issue-reopened': 'This issue was reopened.',
  'pr-merged': 'This pull request was merged.',
  'review': 'A new review was submitted.',
  'review-comment': 'New code review comments were added.',
  'release': 'A new release was published.',
  'push': 'New commits were pushed to this repository.',
};

// initRepoActivityBanner connects to the SSE SharedWorker and shows a
// non-intrusive banner when new activity arrives on the current issue/PR page.
export function initRepoActivityBanner() {
  const pageInfo = parseIssuePageInfo();
  if (!pageInfo.repoId) return; // not on an issue/PR page

  if (notificationSettings.EventSourceUpdateTime <= 0) return;
  if (!window.EventSource || !window.SharedWorker) return;

  const worker = new SharedWorker(
    `${window.__webpack_public_path__}js/eventsource.sharedworker.js?v=${assetVersionEncoded}`,
    'notification-worker',
  );

  worker.port.postMessage({
    type: 'start',
    url: `${window.location.origin}${appSubUrl}/user/events`,
  });

  worker.port.addEventListener('message', (event: MessageEvent<{type: string; data: string}>) => {
    if (event.data?.type !== 'repo-activity') return;

    try {
      const payload = JSON.parse(event.data.data) as {repoID: number; issueIndex: number; eventType: string};
      if (payload.repoID !== pageInfo.repoId) return;

      // If we are on a specific issue page, only show banner for that issue.
      // issueIndex 0 means a repo-wide event (push, release) — always show.
      if (pageInfo.issueNumber && payload.issueIndex && payload.issueIndex !== pageInfo.issueNumber) return;

      showBanner(payload.eventType);
    } catch {
      // ignore malformed events
    }
  });

  worker.port.start();

  window.addEventListener('beforeunload', () => {
    worker.port.postMessage({type: 'close'});
    worker.port.close();
  });
}

function showBanner(eventType: string) {
  const existing = document.querySelector('.repo-activity-banner');
  if (existing) {
    // Update message if a more specific event arrives
    const textEl = existing.querySelector<HTMLSpanElement>('.repo-activity-banner-text');
    if (textEl && eventMessages[eventType]) {
      textEl.textContent = eventMessages[eventType];
    }
    return;
  }

  const message = eventMessages[eventType] ?? 'This page has new activity.';

  const banner = document.createElement('div');
  banner.classList.add('repo-activity-banner');

  const text = document.createElement('span');
  text.classList.add('repo-activity-banner-text');
  text.textContent = message;

  const refreshBtn = document.createElement('button');
  refreshBtn.classList.add('repo-activity-banner-refresh');
  refreshBtn.textContent = 'Refresh';
  refreshBtn.addEventListener('click', () => location.reload());

  const closeBtn = document.createElement('button');
  closeBtn.classList.add('repo-activity-banner-close');
  closeBtn.textContent = '×';
  closeBtn.addEventListener('click', () => banner.remove());

  banner.append(text, refreshBtn, closeBtn);
  document.body.append(banner);
}
