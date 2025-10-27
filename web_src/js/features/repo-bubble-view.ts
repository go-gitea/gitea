import {createApp} from 'vue';
import FishboneGraph from '../components/graph/FishboneGraph.vue';

export async function initRepoBubbleView() {
  const el = document.querySelector('#bubble-view-root');
  if (!el) return;
  if ((el as HTMLElement).getAttribute('data-mounted') === 'true') return;

  // Component is now eagerly imported, eliminating the dynamic import delay
  const app = createApp(FishboneGraph, {
    apiUrl: (el as HTMLElement).getAttribute('data-api-url'),
    owner: (el as HTMLElement).getAttribute('data-owner'),
    repo: (el as HTMLElement).getAttribute('data-repo'),
    subject: (el as HTMLElement).getAttribute('data-subject'),
  });
  app.mount(el);
  (el as HTMLElement).setAttribute('data-mounted', 'true');
}
