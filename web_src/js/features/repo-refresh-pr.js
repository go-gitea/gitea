import $ from 'jquery';

const {appSubUrl} = window.config;

async function receiveBranchUpdated(event) {
  try {
    const data = JSON.parse(event.data);

    const staleBranchAlert = document.querySelector('.refresh-pull-request');

    if (!staleBranchAlert) {
      return;
    }

    const baseTarget = $(staleBranchAlert).data('baseTarget');
    const headTarget = $(staleBranchAlert).data('headTarget');
    const ownerName = $(staleBranchAlert).data('ownerName');
    const repositoryName = $(staleBranchAlert).data('repositoryName');

    if (
      [baseTarget, headTarget].includes(data.Branch) &&
      data.Owner === ownerName &&
      data.Repository === repositoryName
    ) {
      staleBranchAlert.classList.add('active');
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

  if (!!window.EventSource && window.SharedWorker) {
    const worker = new SharedWorker(
      `${__webpack_public_path__}js/eventsource.sharedworker.js`,
      'notification-worker'
    );
    worker.addEventListener('error', (event) => {
      console.error(event);
    });
    worker.port.addEventListener('messageerror', () => {
      console.error('Unable to deserialize message');
    });
    worker.port.postMessage({
      type: 'start',
      url: `${window.location.origin}${appSubUrl}/user/events`,
    });
    worker.port.addEventListener('message', (event) => {
      if (!event.data || !event.data.type) {
        console.error(event);
        return;
      }
      if (event.data.type === 'branch-update') {
        const _promise = receiveBranchUpdated(event.data);
      } else if (event.data.type === 'error') {
        console.error(event.data);
      } else if (event.data.type === 'logout') {
        if (event.data.data !== 'here') {
          return;
        }
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
        window.location.href = appSubUrl;
      } else if (event.data.type === 'close') {
        worker.port.postMessage({
          type: 'close',
        });
        worker.port.close();
      }
    });
    worker.port.addEventListener('error', (e) => {
      console.error(e);
    });
    worker.port.start();
    window.addEventListener('beforeunload', () => {
      worker.port.postMessage({
        type: 'close',
      });
      worker.port.close();
    });
  } else {
    console.error('Service workers not available');
  }
}
