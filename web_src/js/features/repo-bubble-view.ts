import {createApp} from 'vue';

export async function initRepoBubbleView() {
  const el = document.querySelector('#bubble-view-root');
  if (!el) return;

  const {default: FishboneGraph} = await import(/* webpackChunkName: "repo-bubble-view" */'../components/graph/FishboneGraph.vue');
  const app = createApp(FishboneGraph, {
    apiUrl: (el as HTMLElement).getAttribute('data-api-url'),
    owner: (el as HTMLElement).getAttribute('data-owner'),
    repo: (el as HTMLElement).getAttribute('data-repo'),
  });
  // Persist selection when a bubble is focused (listen to a custom event bubbled from component)
  // For now, observe clicks at container level and store owner/subject if available on root element
  const owner = (el as HTMLElement).getAttribute('data-owner') || '';
  const subject = (el as HTMLElement).getAttribute('data-subject') || '';
  el.addEventListener('click', (ev) => {
    const target = ev.target as HTMLElement;
    if (target && target.closest('g.node')) {
      try {
        window.localStorage.setItem('selectedArticleOwner', owner);
        window.localStorage.setItem('selectedArticleSubject', subject);
      } catch { /* ignore */ }
    }
  });
  app.mount(el);
}
