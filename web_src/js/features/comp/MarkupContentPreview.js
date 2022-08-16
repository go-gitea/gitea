import $ from 'jquery';
import {initMarkupContent} from '../../markup/content.js';

const {csrfToken} = window.config;

export function initCompMarkupContentPreviewTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  $tabMenu.find(`.item[data-tab="${$tabMenu.data('preview')}"]`).on('click', function () {
    const $this = $(this);
    $.post($this.data('url'), {
      _csrf: csrfToken,
      mode: 'comment',
      context: $this.data('context'),
      text: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val()
    }, (data) => {
      const $previewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('preview')}"]`);
      $previewPanel.html(data);
      initMarkupContent();
    });
  });
}
