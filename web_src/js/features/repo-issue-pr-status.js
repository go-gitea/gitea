export function initRepoPullRequestCommitStatus() {
  const btn = document.querySelector('.hide-all-checks');
  if (!btn) return;

  btn.addEventListener('click', () => {
    const prCommitStatus = document.querySelector('.pr-commit-status');
    const toggled = prCommitStatus.getAttribute('data-toggled') === 'true';
    btn.textContent = btn.getAttribute(toggled ? 'data-hide-all' : 'data-show-all');
    prCommitStatus.setAttribute('data-toggled', String(!toggled));
  });
}
