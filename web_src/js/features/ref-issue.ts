import {parseIssueHref} from '../utils.ts';
import {GET} from '../modules/fetch.ts';
import {createApp} from 'vue';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';
import type {Issue} from '../types.ts';

type IssueInfo = {
  convertedIssue: Issue,
  renderedLabels: string,
};

const issueInfoCache = new Map<string, IssueInfo>();

async function getIssueInfo(url: string): Promise<IssueInfo> {
  if (issueInfoCache.has(url)) return issueInfoCache.get(url)!;
  const resp = await GET(url);
  if (!resp.ok) throw new Error(resp.statusText || 'Unknown network error');
  const data = await resp.json();
  issueInfoCache.set(url, data);
  return data;
}

async function showRefIssuePopup(link: HTMLAnchorElement) {
  const [data, {default: ContextPopup}] = await Promise.all([
    getIssueInfo(`${link.pathname}/info`),
    import('../components/ContextPopup.vue'),
  ]);
  const el = document.createElement('div');
  const app = createApp(ContextPopup, {
    issue: data.convertedIssue,
    renderedLabels: data.renderedLabels,
  });
  app.mount(el);
  // suppress ancestor title like from .commit-summary to prevent double tooltip
  link.title = '';
  createTippy(link, {
    theme: 'default',
    content: el,
    trigger: 'mouseenter focus',
    placement: 'top-start',
    interactive: true,
    role: 'dialog',
    interactiveBorder: 5,
    onDestroy: () => app.unmount(),
  }).show();
}

export function initRefIssueContextPopup() {
  const selector = 'a[href]:not([data-ref-issue-popup]):not(.ref-external-issue)';
  addDelegatedEventListener<HTMLAnchorElement, MouseEvent>(document, 'mouseover', selector, (link) => {
    if (!parseIssueHref(link.getAttribute('href')!).ownerName) return;
    if (!link.classList.contains('ref-issue') && !link.closest('[data-ref-issue-container]')) return;
    if (getAttachedTippyInstance(link)) return;
    link.setAttribute('data-ref-issue-popup', '');

    // delay so a mouse passing over the link doesn't fire a fetch
    let timer: ReturnType<typeof setTimeout>;
    const cancel = () => {
      clearTimeout(timer);
      link.removeAttribute('data-ref-issue-popup');
      link.removeEventListener('mouseleave', cancel);
    };
    timer = setTimeout(async () => {
      link.removeEventListener('mouseleave', cancel);
      try {
        await showRefIssuePopup(link);
      } catch (err) {
        console.error('Failed to load issue info:', err);
        link.removeAttribute('data-ref-issue-popup');
      }
    }, 300);
    link.addEventListener('mouseleave', cancel);
  });
}
