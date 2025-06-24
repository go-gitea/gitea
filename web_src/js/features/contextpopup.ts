import {createApp} from 'vue';
import ContextPopup from '../components/ContextPopup.vue';
import {parseIssueHref} from '../utils.ts';
import {createTippy} from '../modules/tippy.ts';

export function initContextPopups() {
  const refIssues = document.querySelectorAll<HTMLElement>('.ref-issue');
  attachRefIssueContextPopup(refIssues);
}

export function attachRefIssueContextPopup(refIssues: NodeListOf<HTMLElement>) {
  for (const refIssue of refIssues) {
    if (refIssue.classList.contains('ref-external-issue')) continue;

    const issuePathInfo = parseIssueHref(refIssue.getAttribute('href'));
    if (!issuePathInfo.ownerName) continue;

    const el = document.createElement('div');
    el.classList.add('tw-p-3');
    refIssue.parentNode.insertBefore(el, refIssue.nextSibling);

    const view = createApp(ContextPopup);

    try {
      view.mount(el);
    } catch (err) {
      console.error(err);
      el.textContent = 'ContextPopup failed to load';
    }

    createTippy(refIssue, {
      theme: 'default',
      content: el,
      placement: 'top-start',
      interactive: true,
      role: 'dialog',
      interactiveBorder: 5,
      onShow: () => {
        el.firstChild.dispatchEvent(new CustomEvent('ce-load-context-popup', {detail: issuePathInfo}));
      },
    });
  }
}
