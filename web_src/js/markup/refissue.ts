import {queryElems} from '../utils/dom.ts';
import {parseIssueHref} from '../utils.ts';
import {createApp} from 'vue';
import ContextPopup from '../components/ContextPopup.vue';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';

export function initMarkupRefIssue(el: HTMLElement) {
  queryElems(el, '.ref-issue', (el) => {
    el.addEventListener('mouseenter', showMarkupRefIssuePopup);
    el.addEventListener('focus', showMarkupRefIssuePopup);
  });
}

export function showMarkupRefIssuePopup(e: MouseEvent | FocusEvent) {
  const refIssue = e.currentTarget as HTMLElement;
  if (getAttachedTippyInstance(refIssue)) return;
  if (refIssue.classList.contains('ref-external-issue')) return;

  const issuePathInfo = parseIssueHref(refIssue.getAttribute('href')!);
  if (!issuePathInfo.ownerName) return;

  const el = document.createElement('div');
  const tippy = createTippy(refIssue, {
    theme: 'default',
    content: el,
    trigger: 'mouseenter focus',
    placement: 'top-start',
    interactive: true,
    role: 'dialog',
    interactiveBorder: 5,
    // onHide() { return false }, // help to keep the popup and debug the layout
    onShow: () => {
      const view = createApp(ContextPopup, {
        // backend: GetIssueInfo
        loadIssueInfoUrl: `${window.config.appSubUrl}/${issuePathInfo.ownerName}/${issuePathInfo.repoName}/issues/${issuePathInfo.indexString}/info`,
      });
      view.mount(el);
    },
  });
  tippy.show();
}
