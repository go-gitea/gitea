import {closestDropdown} from '../../modules/dropdown.ts';
import {performFetchActionRequest} from '../../modules/fetch-action.ts';
import {registerGlobalEventFunc} from '../../modules/observer.ts';
import {createElementFromHTML} from '../../utils/dom.ts';

export function initCompReactionSelector() {
  registerGlobalEventFunc('click', 'onCommentReactionButtonClick', async (target: HTMLElement, e: Event) => {
    e.preventDefault();

    if (target.classList.contains('disabled')) return;

    const bottomReactions = (closestDropdown(target)?.container ?? target).closest<HTMLElement>('.bottom-reactions')!;
    const hasReacted = target.getAttribute('data-has-reacted') === 'true';
    const res = await performFetchActionRequest(bottomReactions, {
      method: 'POST',
      url: `${bottomReactions.getAttribute('data-action-url')}/${hasReacted ? 'unreact' : 'react'}`,
      data: new URLSearchParams({content: target.getAttribute('data-reaction-content')!}),
    });
    if (!res) return;

    const data = await res.json();
    const activeElement = document.activeElement;
    const hadFocus = activeElement && bottomReactions.contains(closestDropdown(activeElement)?.container ?? activeElement);
    const newBottomReactions = createElementFromHTML(data.html);
    bottomReactions.replaceWith(newBottomReactions);
    if (hadFocus) newBottomReactions.querySelector<HTMLElement>('.select-reaction')?.focus();
  });
}
