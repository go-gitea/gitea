import {parseIssueHref} from '../utils.ts';
import {GET} from '../modules/fetch.ts';
import {createApp} from 'vue';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';
import type {Issue, IssuePathInfo, TimeoutId} from '../types.ts';

type IssueInfo = {
  convertedIssue: Issue,
  renderedLabels: string,
};

const issueInfoCache = new Map<string, IssueInfo>();

async function getIssueInfo(url: string): Promise<IssueInfo | null> {
  if (issueInfoCache.has(url)) return issueInfoCache.get(url)!;
  const resp = await GET(url);
  // fetch already logs network errors in console, we can ignore network errors here
  if (!resp.ok) return null;
  const data = await resp.json();
  issueInfoCache.set(url, data);
  return data;
}

async function showRefIssuePopup(link: HTMLAnchorElement, info: IssuePathInfo) {
  const [data, {default: ContextPopup}] = await Promise.all([
    getIssueInfo(`${window.config.appSubUrl}/${info.ownerName}/${info.repoName}/${info.pathType}/${info.indexString}/info`),
    import('../components/ContextPopup.vue'),
  ]);
  if (!data) return;

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
    const issuePathInfo = parseIssueHref(link.getAttribute('href')!);
    if (!issuePathInfo) return;
    if (!link.classList.contains('ref-issue') && !link.closest('[data-ref-issue-container]')) return;
    if (getAttachedTippyInstance(link)) return;
    link.setAttribute('data-ref-issue-popup', '');

    // delay so a mouse passing over the link doesn't fire a fetch
    let timer: TimeoutId | undefined;
    const stopDelayedPopup = () => {
      if (!timer) return;
      clearTimeout(timer);
      timer = undefined;
      link.removeAttribute('data-ref-issue-popup');
      link.removeEventListener('mouseleave', stopDelayedPopup);
    };
    timer = setTimeout(async () => {
      link.removeEventListener('mouseleave', stopDelayedPopup);
      try {
        await showRefIssuePopup(link, issuePathInfo);
      } catch (err) {
        console.error('Failed to load issue popup info:', err);
      } finally {
        stopDelayedPopup();
      }
    }, 300);
    link.addEventListener('mouseleave', stopDelayedPopup);
  });
}
