export function initRepoPullRequestCommitStatus() {
  for (const btn of document.querySelectorAll('.commit-status-hide-checks')) {
    btn.addEventListener('click', () => {
      const panel = btn.closest('.commit-status-panel');
      const list = panel.querySelector('.commit-status-list');
      list.style.maxHeight = list.style.maxHeight ? '' : '0px';
      btn.textContent = btn.getAttribute(list.style.maxHeight ? 'data-show-all' : 'data-hide-all');
    });
  }
}
