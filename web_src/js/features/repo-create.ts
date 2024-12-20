const repoName = document.querySelector<HTMLInputElement>('#repo_name');
const repoPublicHint = document.querySelector<HTMLInputElement>('#repo_name_public_profile_hint');
const repoPrivateHint = document.querySelector<HTMLInputElement>('#repo_name_private_profile_hint');

export function initRepoCreate() {
  repoName?.addEventListener('input', () => {
    if (repoName?.value === '.profile') {
      repoPublicHint.style.display = 'inline-block';
    } else {
      repoPublicHint.style.display = 'none';
    }
    if (repoName?.value === '.profile-private') {
      repoPrivateHint.style.display = 'inline-block';
    } else {
      repoPrivateHint.style.display = 'none';
    }
  });
}
