import {createApp} from 'vue';

export async function initRepoBubbleView() {
  const el = document.querySelector('#bubble-view-root');
  if (!el) return;

  const {default: FishboneGraph} = await import(/* webpackChunkName: "repo-bubble-view" */'../components/graph/FishboneGraph.vue');
  createApp(FishboneGraph, {
    apiUrl: (el as HTMLElement).getAttribute('data-api-url'),
    owner: (el as HTMLElement).getAttribute('data-owner'),
    repo: (el as HTMLElement).getAttribute('data-repo'),
  }).mount(el);
}
