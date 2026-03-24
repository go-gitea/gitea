import type {Intent} from '../types.ts';
import {html} from '../utils/html.ts';

export function showGlobalErrorMessage(msg: string, msgType: Intent = 'error') {
  const msgContainer = document.querySelector('.page-content') ?? document.body;
  if (!msgContainer) {
    alert(`${msgType}: ${msg}`);
    return;
  }
  const msgCompact = msg.replace(/\W/g, '').trim(); // compact the message to a data attribute to avoid too many duplicated messages
  let msgDiv = msgContainer.querySelector<HTMLDivElement>(`.js-global-error[data-global-error-msg-compact="${msgCompact}"]`);
  if (!msgDiv) {
    const el = document.createElement('div');
    el.innerHTML = html`<div class="ui container js-global-error tw-my-[--page-spacing]"><div class="ui ${msgType} message tw-text-center tw-whitespace-pre-line"></div></div>`;
    msgDiv = el.childNodes[0] as HTMLDivElement;
  }
  // merge duplicated messages into "the message (count)" format
  const msgCount = Number(msgDiv.getAttribute(`data-global-error-msg-count`)) + 1;
  msgDiv.setAttribute(`data-global-error-msg-compact`, msgCompact);
  msgDiv.setAttribute(`data-global-error-msg-count`, msgCount.toString());
  msgDiv.querySelector('.ui.message')!.textContent = msg + (msgCount > 1 ? ` (${msgCount})` : '');
  msgContainer.prepend(msgDiv);
}
