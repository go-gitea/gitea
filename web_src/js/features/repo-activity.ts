import {parseIssuePageInfo} from '../utils.ts';

const {appSubUrl, notificationSettings, assetVersionEncoded} = window.config;

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
      const payload = JSON.parse(event.data.data) as {repoID: number; issueIndex: number};
      if (payload.repoID !== pageInfo.repoId) return;

      // If we are on a specific issue page, only show banner for that issue.
      // issueIndex 0 means a repo-wide event (new push, new issue) — always show.
      if (pageInfo.issueNumber && payload.issueIndex && payload.issueIndex !== pageInfo.issueNumber) return;

      showBanner();
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

function showBanner() {
  if (document.getElementById('gitea-repo-activity-banner')) return;

  const banner = document.createElement('div');
  banner.id = 'gitea-repo-activity-banner';
  Object.assign(banner.style, {
    position: 'fixed',
    top: '64px',
    left: '50%',
    transform: 'translateX(-50%)',
    background: 'var(--color-primary)',
    color: 'var(--color-primary-contrast)',
    padding: '8px 16px',
    borderRadius: '6px',
    zIndex: '9999',
    display: 'flex',
    alignItems: 'center',
    gap: '12px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.25)',
    fontSize: '14px',
  });

  const text = document.createElement('span');
  text.textContent = 'This page has new activity.';

  const refreshBtn = document.createElement('button');
  refreshBtn.textContent = 'Refresh';
  Object.assign(refreshBtn.style, {
    background: 'rgba(255,255,255,0.2)',
    border: 'none',
    color: 'inherit',
    padding: '3px 10px',
    borderRadius: '4px',
    cursor: 'pointer',
    fontWeight: 'bold',
  });
  refreshBtn.addEventListener('click', () => location.reload());

  const closeBtn = document.createElement('button');
  closeBtn.textContent = '×';
  Object.assign(closeBtn.style, {
    background: 'none',
    border: 'none',
    color: 'inherit',
    cursor: 'pointer',
    fontSize: '18px',
    lineHeight: '1',
    padding: '0',
  });
  closeBtn.addEventListener('click', () => banner.remove());

  banner.append(text, refreshBtn, closeBtn);
  document.body.appendChild(banner);
}
