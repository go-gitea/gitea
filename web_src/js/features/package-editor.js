import $ from 'jquery';

const {csrfToken} = window.config;

function initEditPreviewTab($form) {
  const $tabMenu = $form.find('.tabular.menu');
  $tabMenu.find('.item').tab();
  $tabMenu.find(`.item[data-tab="${$tabMenu.data('preview')}"]`).on('click', function () {
    const $this = $(this);
    const context = `/${$this.data('context')}`;
    const mode = $this.data('markdown-mode') || 'comment';

    $.post($this.data('url'), {
      _csrf: csrfToken,
      mode,
      context,
      text: $form.find(`.tab[data-tab="${$tabMenu.data('write')}"] textarea`).val(),
    }, (data) => {
      const $diffPreviewPanel = $form.find(`.tab[data-tab="${$tabMenu.data('preview')}"]`);
      $diffPreviewPanel.html(data);
    });
  });
}

export function initPackageEditor() {
  if ($('.package.settings .descriptions').length === 0) {
    return;
  }

  initEditPreviewTab($('.package.settings .descriptions'));
}
