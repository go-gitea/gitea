import {performFetchActionRequest} from '../../modules/fetch-action.ts';
import {registerGlobalEventFunc} from '../../modules/observer.ts';
import {createTippy} from '../../modules/tippy.ts';
import {createElementFromHTML} from '../../utils/dom.ts';

export function initCompReactionSelector() {
  registerGlobalEventFunc('click', 'onReactionPickerClick', (btn: HTMLElement) => {
    if (btn._tippy) return;
    createTippy(btn, {
      content: btn.nextElementSibling!,
      theme: 'menu',
      arrow: false,
      maxWidth: 'none',
      placement: 'bottom-start',
      trigger: 'click',
      interactive: true,
      hideOnClick: true,
    }).show();
  });

  registerGlobalEventFunc('click', 'onCommentReactionButtonClick', async (target: HTMLElement, e: Event) => {
    e.preventDefault();

    if (target.classList.contains('disabled')) return;

    const picker = target.closest('[data-tippy-root]')?._tippy;
    picker?.hide();
    const bottomReactions = (picker?.reference ?? target).closest<HTMLElement>('.bottom-reactions')!;
    const hasReacted = target.getAttribute('data-has-reacted') === 'true';
    const res = await performFetchActionRequest(bottomReactions, {
      method: 'POST',
      url: `${bottomReactions.getAttribute('data-action-url')}/${hasReacted ? 'unreact' : 'react'}`,
      data: new URLSearchParams({content: target.getAttribute('data-reaction-content')!}),
    });
    if (!res) return;

    const data = await res.json();
    const hadFocus = bottomReactions.contains(document.activeElement);
    bottomReactions.querySelector('.select-reaction')?._tippy?.destroy();
    const newBottomReactions = createElementFromHTML(data.html);
    bottomReactions.replaceWith(newBottomReactions);
    if (hadFocus) newBottomReactions.querySelector<HTMLElement>('.select-reaction')?.focus();
  });
}
