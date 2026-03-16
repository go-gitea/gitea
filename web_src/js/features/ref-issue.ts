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

export function initRefIssueContextPopup() {
  addDelegatedEventListener(document, 'mouseover', 'a[href]:not([data-ref-issue-popup])', (link: HTMLElement) => {
    const href = link.getAttribute('href')!;
    if (!parseIssueHref(href).ownerName) return; // not an issue/PR link
    if (link.closest('.ref-issue-popup')) return; // avoid nesting
    if (link.classList.contains('ref-external-issue')) return; // external tracker
    if (getAttachedTippyInstance(link)) return; // already has tooltip
    link.setAttribute('data-ref-issue-popup', ''); // prevent parallel fetches

    const infoUrl = `${new URL(href, window.location.origin).pathname}/info`;
    (async () => {
      try {
        const [data, {default: ContextPopup}] = await Promise.all([
          getIssueInfo(infoUrl),
          import(/* webpackChunkName: "ContextPopup" */ '../components/ContextPopup.vue'),
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
      } catch (err) {
        console.error('Failed to load issue info:', err);
        link.removeAttribute('data-ref-issue-popup');
      }
    })();
  });
}
