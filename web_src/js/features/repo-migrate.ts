import {hideElem, showElem} from '../utils/dom.ts';
import {GET, POST} from '../modules/fetch.ts';

export function initRepoMigrationStatusChecker() {
  const repoMigrating = document.querySelector('#repo_migrating');
  if (!repoMigrating) return;

  document.querySelector('#repo_migrating_retry')?.addEventListener('click', doMigrationRetry);

  const repoLink = repoMigrating.getAttribute('data-migrating-repo-link');

  // returns true if the refresh still needs to be called after a while
  const refresh = async () => {
    const res = await GET(`${repoLink}/-/migrate/status`);
    if (res.status !== 200) return true; // continue to refresh if network error occurs

    const data = await res.json();

    // for all status
    if (data.message) {
      document.querySelector('#repo_migrating_progress_message').textContent = data.message;
    }

    // TaskStatusFinished
    if (data.status === 4) {
      window.location.reload();
      return false;
    }

    // TaskStatusFailed
    if (data.status === 3) {
      hideElem('#repo_migrating_progress');
      hideElem('#repo_migrating');
      showElem('#repo_migrating_retry');
      showElem('#repo_migrating_failed');
      showElem('#repo_migrating_failed_image');
      document.querySelector('#repo_migrating_failed_error').textContent = data.message;
      return false;
    }

    return true; // continue to refresh
  };

  const syncTaskStatus = async () => {
    let doNextRefresh = true;
    try {
      doNextRefresh = await refresh();
    } finally {
      if (doNextRefresh) {
        setTimeout(syncTaskStatus, 2000);
      }
    }
  };

  syncTaskStatus(); // no await
}

async function doMigrationRetry(e) {
  await POST(e.target.getAttribute('data-migrating-task-retry-url'));
  window.location.reload();
}
