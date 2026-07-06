import {createApp} from 'vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';

async function initRepoAIReview(el: HTMLElement) {
  const container = el.querySelector('#ai-review-status');
  if (!container) return;
  const statusUrl = container.getAttribute('data-status-url');
  if (!statusUrl) return;
  const {default: AIRreviewStatus} = await import('../components/AIRreviewStatus.vue');
  const view = createApp(AIRreviewStatus, {statusUrl});
  view.mount(container);
}

export function initRepoAIReviewOnPage() {
  registerGlobalInitFunc('initRepoAIReview', initRepoAIReview);
}
