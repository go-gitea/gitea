import ContextPopup from '../components/ContextPopup.vue';
import {createVueRoot} from '../utils/vue.js';
import {parseIssueHref} from '../utils.js';
import {createTippy} from '../modules/tippy.js';
import {GET} from '../modules/fetch.js';

const {appSubUrl} = window.config;

async function attach(e) {
  const link = e.currentTarget;

  // ignore external issues
  if (link.classList.contains('ref-external-issue')) return;
  // ignore links that are already loading
  if (link.hasAttribute('data-issue-ref-loading')) return;

  const {owner, repo, index} = parseIssueHref(link.getAttribute('href'));
  if (!owner) return;

  const url = `${appSubUrl}/${owner}/${repo}/issues/${index}/info`; // backend: GetIssueInfo
  if (link.getAttribute('data-issue-ref-info-url') === url) return; // link already has a tooltip with this url

  try {
    link.setAttribute('data-issue-ref-loading', 'true');
    let res;
    try {
      res = await GET(url);
    } catch {}
    if (!res.ok) return;

    let issue, labelsHtml;
    try {
      ({issue, labelsHtml} = await res.json());
    } catch {}
    if (!issue) return;

    const repoUrl = `${appSubUrl}/${owner}/${repo}`;
    const content = createVueRoot(ContextPopup, {issue, labelsHtml, repoUrl});
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
    link.setAttribute('data-issue-ref-info-url', url);

    // show immediately because this runs during mouseenter and focus
    tippy.show();
  } finally {
    link.removeAttribute('data-issue-ref-loading');
  }
}

export function attachRefIssueContextPopup(els) {
  for (const el of els) {
    el.addEventListener('mouseenter', attach);
    el.addEventListener('focus', attach);
  }
}

export function initContextPopups() {
  // TODO: Use MutationObserver to detect newly inserted .ref-issue
  attachRefIssueContextPopup(document.querySelectorAll('.ref-issue'));
}
