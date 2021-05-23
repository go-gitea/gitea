import {svg} from '../svg.js';

export function showLineButton() {
  if ($('.code-line-menu').length === 0) return;
  $('.code-line-button').remove();
  $('.code-view td.lines-code.active').closest('tr').find('td:eq(0)').first().prepend(
    $(`<button class="code-line-button">${svg('octicon-kebab-horizontal')}</button>`)
  );
  $('.code-line-menu').appendTo($('.code-view'));
  $('.code-line-button').popup({popup: $('.code-line-menu'), on: 'click'});
}
