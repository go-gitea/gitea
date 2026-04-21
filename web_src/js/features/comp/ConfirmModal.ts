import {svg} from '../../svg.ts';
import {html, htmlRaw} from '../../utils/html.ts';
import {createElementFromHTML} from '../../utils/dom.ts';
import {fomanticQuery} from '../../modules/fomantic/base.ts';
import {hideToastsAll} from '../../modules/toast.ts';

const {i18n} = window.config;

type ConfirmModalOptions = {
  header?: string;
  content?: string;
  confirmButtonColor?: 'primary' | 'red' | 'green' | 'blue';
};

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
  // hide existing toasts when we need to show a new modal, otherwise the toasts only interfere the UI
  // it's fine to do so because the modal is triggered by user's explicit action, so the user should already have read the toast messages
  hideToastsAll();
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
