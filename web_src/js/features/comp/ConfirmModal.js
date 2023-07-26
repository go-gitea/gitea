import $ from 'jquery';
import {svg} from '../../svg.js';
import {htmlEscape} from 'escape-goat';

const {i18n} = window.config;

export async function confirmModal(opts = {content: '', buttonColor: 'green'}) {
  return new Promise((resolve) => {
    const $modal = $(`
<div class="ui g-modal-confirm modal">
  <div class="content">${htmlEscape(opts.content)}</div>
  <div class="actions">
    <button class="ui basic cancel button">${svg('octicon-x')} ${i18n.modal_cancel}</button>
    <button class="ui ${opts.buttonColor || 'green'} ok button">${svg('octicon-check')} ${i18n.modal_confirm}</button>
  </div>
</div>
`);

    $modal.appendTo(document.body);
    $modal.modal({
      onApprove() {
        resolve(true);
      },
      onHidden() {
        $modal.remove();
        resolve(false);
      },
    }).modal('show');
  });
}
