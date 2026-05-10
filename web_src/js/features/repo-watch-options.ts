import {fomanticQuery} from '../modules/fomantic/base.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {submitFormFetchAction} from './common-fetch-action.ts';

export function initRepoWatchOptions() {
  const elModal = document.querySelector<HTMLElement>('#repo-watch-options-modal');
  if (!elModal) return;

  const elPullRequests = elModal.querySelector<HTMLInputElement>('[name="pull_requests"]')!;
  const elIssues = elModal.querySelector<HTMLInputElement>('[name="issues"]')!;
  const elReleases = elModal.querySelector<HTMLInputElement>('[name="releases"]')!;

  const showModal = (btn: HTMLElement) => {
    const form = elModal.querySelector<HTMLFormElement>('form')!;
    elPullRequests.checked = btn.getAttribute('data-watch-pull-requests') === 'true';
    elIssues.checked = btn.getAttribute('data-watch-issues') === 'true';
    elReleases.checked = btn.getAttribute('data-watch-releases') === 'true';
    form.action = `${btn.getAttribute('data-repo-link')!}/action/watch/options`;

    fomanticQuery(elModal).modal({
      autofocus: false,
      onApprove() {
        submitFormFetchAction(form);
        return false;
      },
    }).modal('show');
  };

  registerGlobalInitFunc('initWatchOptions', (el: HTMLElement) => {
    el.addEventListener('click', (e) => {
      e.preventDefault();
      showModal(el);
    });
  });
}
