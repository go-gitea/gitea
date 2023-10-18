export function initRepoPullRequestCommitStatus() {
  const btn = document.querySelector('.hide-all-checks');
  const prCommitStatus = document.querySelector('.pr-commit-status');
  if (!btn) return;
  btn.addEventListener('click', () => {
    if (prCommitStatus.classList.contains('hide')) {
      btn.textContent = btn.getAttribute('data-hide-all');
    } else {
      btn.textContent = btn.getAttribute('data-show-all');
    }
    prCommitStatus.classList.toggle('hide');
  });
}
