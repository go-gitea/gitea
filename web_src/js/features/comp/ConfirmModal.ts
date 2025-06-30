import {svg} from '../../svg.ts';
import {htmlEscape} from 'escape-goat';
import {createElementFromHTML} from '../../utils/dom.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';

const {i18n} = window.config;

type ConfirmModalOptions = {
  header?: string;
  content?: string;
  confirmButtonColor?: 'primary' | 'red' | 'green' | 'blue';
}

export function createConfirmModal({header = '', content = '', confirmButtonColor = 'primary'}:ConfirmModalOptions = {}): HTMLElement {
  const headerHtml = header ? `<div class="header">${htmlEscape(header)}</div>` : '';
  return createElementFromHTML(`
<div class="ui g-modal-confirm modal">
  ${headerHtml}
  <div class="content">${htmlEscape(content)}</div>
  <div class="actions">
    <button class="ui cancel button">${svg('octicon-x')} ${htmlEscape(i18n.modal_cancel)}</button>
    <button class="ui ${confirmButtonColor} ok button">${svg('octicon-check')} ${htmlEscape(i18n.modal_confirm)}</button>
  </div>
</div>
`);
}

export function confirmModal(modal: HTMLElement | ConfirmModalOptions): Promise<boolean> {
  if (!(modal instanceof HTMLElement)) modal = createConfirmModal(modal);
  return new Promise((resolve) => {
    const $modal = fomanticQuery(modal);
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
