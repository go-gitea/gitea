import {validateTextareaNonEmpty} from './comp/ComboMarkdownEditor.ts';
import {showErrorToast} from '../modules/toast.ts';
import {addDelegatedEventListener, createElementFromHTML} from '../utils/dom.ts';
import {errorMessage} from '../modules/errors.ts';
import {POST} from '../modules/fetch.ts';

// the reply forms and the "resolve" buttons of the code comment conversations in the PR timeline
export function initRepoDiffConversationForm() {
  // FIXME: there could be various different form in a conversation-holder (for example: reply form, edit form).
  // This listener is for "reply form" only, it should clearly distinguish different forms in the future.
  addDelegatedEventListener<HTMLFormElement, SubmitEvent>(document, 'submit', '.conversation-holder form', async (form, e) => {
    e.preventDefault();
    const textArea = form.querySelector<HTMLTextAreaElement>('textarea')!;
    if (!validateTextareaNonEmpty(textArea)) return;
    if (form.classList.contains('is-loading')) return;

    try {
      form.classList.add('is-loading');
      const response = await POST(form.getAttribute('action')!, {data: new FormData(form)});
      if (!response.ok) throw new Error((await response.json()).errorMessage);
      const newConversationHolder = createElementFromHTML(await response.text());
      form.closest('.conversation-holder')!.replaceWith(newConversationHolder);
      (form as any) = null; // prevent further usage of the form because it should have been replaced
    } catch (error) {
      console.error('Error:', error);
      showErrorToast(`Submit form failed: ${errorMessage(error)}`);
    } finally {
      form?.classList.remove('is-loading');
    }
  });

  addDelegatedEventListener(document, 'click', '.resolve-conversation', async (el, e) => {
    e.preventDefault();
    const comment_id = el.getAttribute('data-comment-id')!;
    const origin = el.getAttribute('data-origin')!;
    const action = el.getAttribute('data-action')!;
    const url = el.getAttribute('data-update-url')!;

    try {
      const response = await POST(url, {data: new URLSearchParams({origin, action, comment_id})});
      const data = await response.text();

      const elConversationHolder = el.closest('.conversation-holder');
      if (elConversationHolder) {
        const elNewConversation = createElementFromHTML(data);
        elConversationHolder.replaceWith(elNewConversation);
      } else {
        window.location.reload();
      }
    } catch (error) {
      console.error('Error:', error);
    }
  });
}
