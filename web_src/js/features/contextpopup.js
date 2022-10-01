import $ from 'jquery';
import {createApp} from 'vue';
import ContextPopup from '../components/ContextPopup.vue';
import {parseIssueHref} from '../utils.js';
import {createTippy} from '../modules/tippy.js';

export default function initContextPopups() {
  const refIssues = $('.ref-issue');
  if (!refIssues.length) return;

  refIssues.each(function () {
    if ($(this).hasClass('ref-external-issue')) {
      return;
    }

    const {owner, repo, index} = parseIssueHref($(this).attr('href'));
    if (!owner) return;

    const el = document.createElement('div');
    this.parentNode.insertBefore(el, this.nextSibling);

    const view = createApp(ContextPopup);

    try {
      view.mount(el);
    } catch (err) {
      console.error(err);
      el.textContent = 'ContextPopup failed to load';
    }

    createTippy(this, {
      content: el,
      interactive: true,
      onShow: () => {
        el.firstChild.dispatchEvent(new CustomEvent('us-load-context-popup', {detail: {owner, repo, index}}));
      }
    });
  });
}
