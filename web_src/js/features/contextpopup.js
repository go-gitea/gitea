import ContextPopup from '../components/ContextPopup.vue';
import {createVueRoot} from '../utils/vue.js';
import {parseIssueHref} from '../utils.js';
import {createTippy} from '../modules/tippy.js';
import {GET} from '../modules/fetch.js';

const {appSubUrl} = window.config;
const urlAttribute = 'data-issue-ref-url';

async function init(e) {
  const link = e.currentTarget;

  const {owner, repo, index} = parseIssueHref(link.getAttribute('href'));
  if (!owner) return;

  const url = `${appSubUrl}/${owner}/${repo}/issues/${index}/info`; // backend: GetIssueInfo
  if (link.getAttribute(urlAttribute) === url) return; // link already has a tooltip with this url

  const res = await GET(url);
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
    role: 'tooltip',
    interactiveBorder: 15,
  });

  // set attribute on the link that indicates which url the tooltip currently renders
  link.setAttribute(urlAttribute, url);

  // show immediately because this runs during mouseenter and focus
  tippy.show();
}

export function attachRefIssueContextPopup(els) {
  for (const el of els) {
    el.addEventListener('mouseenter', init);
    el.addEventListener('focus', init);
  }
}

export function initContextPopups() {
  // TODO: Use MutationObserver to detect newly inserted .ref-issue
  attachRefIssueContextPopup(document.querySelectorAll('.ref-issue:not(.ref-external-issue)'));
}
