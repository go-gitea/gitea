const repoName = document.querySelector<HTMLInputElement>('#repo_name');
const repoPublicHint = document.querySelector<HTMLInputElement>('#repo_name_public_profile_hint');
const repoPrivateHint = document.querySelector<HTMLInputElement>('#repo_name_private_profile_hint');

export function initRepoCreate() {
  repoName?.addEventListener('input', () => {
    if (repoName?.value === '.profile') {
      repoPublicHint.classList.remove('tw-hidden');
    } else {
      repoPublicHint.classList.add('tw-hidden');
    }
    if (repoName?.value === '.profile-private') {
      repoPrivateHint.classList.remove('tw-hidden');
    } else {
      repoPrivateHint.classList.add('tw-hidden');
    }
  });
}
