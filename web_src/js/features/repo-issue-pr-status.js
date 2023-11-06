export function initRepoPullRequestCommitStatus() {
  for (const btn of document.querySelectorAll('.commit-status-hide-checks')) {
    const panel = btn.closest('.commit-status-panel');
    const list = panel.querySelector('.commit-status-list');
    const header = panel.querySelector('.commit-status-header');
    let hasScrollBar = false;
    btn.addEventListener('click', () => {
      if (!hasScrollBar && !list.style.overflow) {
        hasScrollBar = list.scrollHeight > list.clientHeight;
      }
      // hide the flickering scrollbar during transition if there was no scrollbar
      list.style.overflow = hasScrollBar ? '' : 'hidden';
      list.style.maxHeight = list.style.maxHeight ? '' : '0px'; // toggle
      if (!list.style.maxHeight) header.style.borderBottom = '';
      btn.textContent = btn.getAttribute(list.style.maxHeight ? 'data-show-all' : 'data-hide-all');
    });
    list.addEventListener('transitionend', () => {
      list.style.overflow = '';
      header.style.borderBottom = list.style.maxHeight ? 'none' : '';
    });
  }
}
