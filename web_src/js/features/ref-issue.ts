import {parseIssueHref} from '../utils.ts';
import {GET} from '../modules/fetch.ts';
import {createApp} from 'vue';
import {createTippy, getAttachedTippyInstance} from '../modules/tippy.ts';
import {addDelegatedEventListener} from '../utils/dom.ts';

const issueInfoCache = new Map<string, Promise<any>>();

function getIssueInfo(url: string): Promise<any> {
  let promise = issueInfoCache.get(url);
  if (!promise) {
    promise = (async () => {
      try {
        const resp = await GET(url);
        if (!resp.ok) throw new Error(resp.statusText || 'Unknown network error');
        return await resp.json();
      } catch (err) {
        issueInfoCache.delete(url);
        throw err;
      }
    })();
    issueInfoCache.set(url, promise);
  }
  return promise;
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
    if (link.closest('[data-no-ref-issue-popup]')) return;
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
