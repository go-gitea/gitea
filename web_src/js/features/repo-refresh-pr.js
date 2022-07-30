import $ from 'jquery';
import getMemoizedSharedWorker from './shared-worker.js';

async function receiveBranchUpdated(event) {
  try {
    const data = JSON.parse(event.data);

    const refreshPullRequest = document.querySelector('.refresh-pull-request');

    if (!refreshPullRequest) {
      return;
    }

    const baseTarget = $(refreshPullRequest).data('baseTarget');
    const headTarget = $(refreshPullRequest).data('headTarget');
    const ownerName = $(refreshPullRequest).data('ownerName');
    const repositoryName = $(refreshPullRequest).data('repositoryName');

    if (
      [baseTarget, headTarget].includes(data.Branch) &&
      data.Owner === ownerName &&
      data.Repository === repositoryName
    ) {
      refreshPullRequest.classList.add('active');
    }
  } catch (error) {
    console.error(error, event);
  }
}

export function initRepoRefreshPullRequest() {
  const staleBranchAlert = $('.refresh-pull-request');

  if (!staleBranchAlert.length) {
    return;
  }

  $(staleBranchAlert).on('click', () => {
    window.location.reload();
  });

  const worker = getMemoizedSharedWorker();

  if (worker) {
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        return;
      }
      if (event.data.type === 'branch-update') {
        receiveBranchUpdated(event.data);
      }
    });
  } else {
    console.error('Service workers not available');
  }
}
