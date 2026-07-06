import {createApp} from 'vue';
import {registerGlobalInitFunc} from '../modules/observer.ts';

function initRepoAIReview(el: HTMLElement) {
  const container = el.querySelector('#ai-review-status');
  if (!container) return;
  const statusUrl = container.getAttribute('data-status-url');
  if (!statusUrl) return;
  import('../components/AIRreviewStatus.vue').then(({default: AIRreviewStatus}) => {
    const view = createApp(AIRreviewStatus, {statusUrl});
    view.mount(container);
  });
}

export function initRepoAIReviewOnPage() {
  registerGlobalInitFunc('initRepoAIReview', initRepoAIReview);
}
