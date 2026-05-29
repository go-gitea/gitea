import {fomanticQuery} from '../modules/fomantic/base.ts';
import {registerGlobalInitFunc} from '../modules/observer.ts';
import {submitFormFetchAction} from './common-fetch-action.ts';

export function initRepoWatch() {
  const elModal = document.querySelector<HTMLElement>('#repo-watch-options-modal');
  if (!elModal) return;

  const form = elModal.querySelector<HTMLFormElement>('form')!;
  const elPullRequests = elModal.querySelector<HTMLInputElement>('[name="pull_requests"]')!;
  const elIssues = elModal.querySelector<HTMLInputElement>('[name="issues"]')!;
  const elReleases = elModal.querySelector<HTMLInputElement>('[name="releases"]')!;
  const elSubmitBtn = elModal.querySelector<HTMLButtonElement>('.approve.button')!;

  const updateFormValidity = () => {
    const isValid = elPullRequests.checked || elIssues.checked || elReleases.checked;
    elPullRequests.setCustomValidity(isValid ? '' : form.getAttribute('data-required-message')!);
    elSubmitBtn.disabled = !isValid;
    form.reportValidity();
  };

  const resetModalInputs = (btn: HTMLElement) => {
    elPullRequests.checked = btn.getAttribute('data-watch-pull-requests') === 'true';
    elIssues.checked = btn.getAttribute('data-watch-issues') === 'true';
    elReleases.checked = btn.getAttribute('data-watch-releases') === 'true';
    updateFormValidity();
  };

  elPullRequests.addEventListener('change', updateFormValidity);
  elIssues.addEventListener('change', updateFormValidity);
  elReleases.addEventListener('change', updateFormValidity);

  const showWatchOptionsModal = (btn: HTMLElement, action: string, sync: string) => {
    resetModalInputs(btn);
    form.action = action;
    form.setAttribute('data-fetch-sync', sync);

    fomanticQuery(elModal).modal({
      autofocus: false,
      async onApprove() {
        if (!form.reportValidity()) return false;

        try {
          await submitFormFetchAction(form);
        } finally {
          form.removeAttribute('data-fetch-sync');
        }
        return false;
      },
    }).modal('show');
  };

  registerGlobalInitFunc('initWatchCustom', (btn: HTMLElement) => {
    btn.addEventListener('click', () => showWatchOptionsModal(
      btn,
      `${btn.getAttribute('data-repo-link')!}/action/watch`,
      '$body #repo-header-watch',
    ));
  });

  registerGlobalInitFunc('initWatchOptions', (btn: HTMLElement) => {
    btn.addEventListener('click', () => showWatchOptionsModal(
      btn,
      `${btn.getAttribute('data-repo-link')!}/action/watch/options`,
      `$body #${btn.id}`,
    ));
  });
}
