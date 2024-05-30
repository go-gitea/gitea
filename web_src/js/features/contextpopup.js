import ContextPopup from '../components/ContextPopup.vue';
import {createVueRoot} from '../utils/vue.js';
import {parseIssueHref} from '../utils.js';
import {createTippy} from '../modules/tippy.js';
import {GET} from '../modules/fetch.js';

const {appSubUrl} = window.config;

async function show(e) {
  const link = e.currentTarget;
  const {owner, repo, index} = parseIssueHref(link.getAttribute('href'));
  if (!owner) return;

  const res = await GET(`${appSubUrl}/${owner}/${repo}/issues/${index}/info`); // backend: GetIssueInfo
  if (!res.ok) return;

  let issue, labelsHtml;
  try {
    ({issue, labelsHtml} = await res.json());
  } catch {}
  if (!issue) return;

  const content = createVueRoot(ContextPopup, {issue, labelsHtml});
  if (!content) return;

  const tippy = createTippy(link, {
    theme: 'default',
    trigger: 'mouseenter focus',
    content,
    placement: 'top-start',
    interactive: true,
    role: 'dialog',
    interactiveBorder: 15,
  });

  // show immediately because this runs during mouseenter and focus
  tippy.show();
}

export function attachRefIssueContextPopup(els) {
  for (const link of els) {
    link.addEventListener('mouseenter', show);
    link.addEventListener('focus', show);
  }
}

export function initContextPopups() {
  // TODO: Use MutationObserver to detect newly inserted .ref-issue
  attachRefIssueContextPopup(document.querySelectorAll('.ref-issue:not(.ref-external-issue)'));
}
