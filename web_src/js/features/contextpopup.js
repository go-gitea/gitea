import $ from 'jquery';
import Vue from 'vue';
import ContextPopup from '../components/ContextPopup.vue';
import {parseIssueHref} from '../utils.js';
import {createPopup} from './popup.js';

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
    el.innerHTML = '<div></div>';
    this.parentNode.insertBefore(el, this.nextSibling);

    const View = Vue.extend({
      render: (createElement) => createElement(ContextPopup),
    });

    const view = new View();

    try {
      view.$mount(el.firstChild);
    } catch (err) {
      console.error(err);
      el.textContent = 'ContextPopup failed to load';
    }

    createPopup(this, {
      content: el,
      interactive: true,
      onShow: () => {
        view.$emit('load-context-popup', {owner, repo, index});
      }
    });
  });
}
