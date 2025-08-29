import {svg} from '../../svg.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {createElementFromHTML} from '../../utils/dom.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';

const {i18n} = window.config;

type ConfirmModalOptions = {
  header?: string;
  content?: string;
  confirmButtonColor?: 'primary' | 'red' | 'green' | 'blue';
}

export function createConfirmModal({header = '', content = '', confirmButtonColor = 'primary'}:ConfirmModalOptions = {}): HTMLElement {
  const headerHtml = header ? html`<div class="header">${header}</div>` : '';
  return createElementFromHTML(html`
    <div class="ui g-modal-confirm modal">
      ${htmlRaw(headerHtml)}
      <div class="content">${content}</div>
      <div class="actions">
        <button class="ui cancel button">${htmlRaw(svg('octicon-x'))} ${i18n.modal_cancel}</button>
        <button class="ui ${confirmButtonColor} ok button">${htmlRaw(svg('octicon-check'))} ${i18n.modal_confirm}</button>
      </div>
    </div>
  `.trim());
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
