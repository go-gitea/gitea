export function initRepoPullRequestCommitStatus() {
  for (const btn of document.querySelectorAll('.commit-status-hide-checks')) {
    const panel = btn.closest('.commit-status-panel');
    const list = panel.querySelector('.commit-status-list');
    let hasScrollBar = null;
    btn.addEventListener('click', () => {
      if (hasScrollBar === null && !list.style.overflow) {
        hasScrollBar = list.scrollHeight > list.clientHeight;
      }
      list.style.overflow = hasScrollBar ? '' : 'hidden'; // hide the flickering scrollbar when hiding if there was no scrollbar
      list.style.maxHeight = list.style.maxHeight ? '' : '0px'; // toggle
      btn.textContent = btn.getAttribute(list.style.maxHeight ? 'data-show-all' : 'data-hide-all');
    });
    list.addEventListener('transitionend', () => list.style.overflow = '');
  }
}
