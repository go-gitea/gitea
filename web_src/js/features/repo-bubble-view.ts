import {createApp} from 'vue';

export async function initRepoBubbleView() {
  const el = document.querySelector('#bubble-view-root');
  if (!el) return;
  if ((el as HTMLElement).dataset.mounted === 'true') return;

  const {default: FishboneGraph} = await import(/* webpackChunkName: "repo-bubble-view" */'../components/graph/FishboneGraph.vue');
  const app = createApp(FishboneGraph, {
    apiUrl: (el as HTMLElement).getAttribute('data-api-url'),
    owner: (el as HTMLElement).getAttribute('data-owner'),
    repo: (el as HTMLElement).getAttribute('data-repo'),
    subject: (el as HTMLElement).getAttribute('data-subject'),
  });
  app.mount(el);
  (el as HTMLElement).dataset.mounted = 'true';
}
