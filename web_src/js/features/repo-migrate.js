import $ from 'jquery';
import {hideElem, showElem} from '../utils/dom.js';
import {GET, POST} from '../modules/fetch.js';

const {appSubUrl} = window.config;

export function initRepoMigrationStatusChecker() {
  const $repoMigrating = $('#repo_migrating');
  if (!$repoMigrating.length) return;

  $('#repo_migrating_retry').on('click', doMigrationRetry);

  const task = $repoMigrating.attr('data-migrating-task-id');

  // returns true if the refresh still need to be called after a while
  const refresh = async () => {
    const res = await GET(`${appSubUrl}/user/task/${task}`);
    if (res.status !== 200) return true; // continue to refresh if network error occurs

    const data = await res.json();

    // for all status
    if (data.message) {
      $('#repo_migrating_progress_message').text(data.message);
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
      $('#repo_migrating_failed_error').text(data.message);
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
  await POST($(e.target).attr('data-migrating-task-retry-url'));
  window.location.reload();
}
