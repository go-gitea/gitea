import {handleReply} from './repo-issue.ts';
import {getComboMarkdownEditor, initComboMarkdownEditor, ComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {hideElem, querySingleVisibleElem, showElem} from '../utils/dom.ts';
import {attachRefIssueContextPopup} from './contextpopup.ts';
import {initCommentContent, initMarkupContent} from '../markup/content.ts';
import {triggerUploadStateChanged} from './comp/EditorUpload.ts';
import {convertHtmlToMarkdown} from '../markup/html2markdown.ts';
import {applyAreYouSure, reinitializeAreYouSure} from '../vendor/jquery.are-you-sure.ts';

async function tryOnEditContent(e) {
  const clickTarget = e.target.closest('.edit-content');
  if (!clickTarget) return;

  e.preventDefault();
  const segment = clickTarget.closest('.header').nextElementSibling;
  const editContentZone = segment.querySelector('.edit-content-zone');
  const renderContent = segment.querySelector('.render-content');
  const rawContent = segment.querySelector('.raw-content');

  let comboMarkdownEditor : ComboMarkdownEditor;

  const cancelAndReset = (e) => {
    e.preventDefault();
    showElem(renderContent);
    hideElem(editContentZone);
    comboMarkdownEditor.dropzoneReloadFiles();
  };

  const saveAndRefresh = async (e) => {
    e.preventDefault();
    // we are already in a form, do not bubble up to the document otherwise there will be other "form submit handlers"
    // at the moment, the form submit event conflicts with initRepoDiffConversationForm (global '.conversation-holder form' event handler)
    e.stopPropagation();
    renderContent.classList.add('is-loading');
    showElem(renderContent);
    hideElem(editContentZone);
    try {
      const params = new URLSearchParams({
        content: comboMarkdownEditor.value(),
        context: editContentZone.getAttribute('data-context'),
        content_version: editContentZone.getAttribute('data-content-version'),
      });
      for (const file of comboMarkdownEditor.dropzoneGetFiles() ?? []) {
        params.append('files[]', file);
      }

      const response = await POST(editContentZone.getAttribute('data-update-url'), {data: params});
      const data = await response.json();
      if (response.status === 400) {
        showErrorToast(data.errorMessage);
        return;
      }
      reinitializeAreYouSure(editContentZone.querySelector('form')); // the form is no longer dirty
      editContentZone.setAttribute('data-content-version', data.contentVersion);
      if (!data.content) {
        renderContent.innerHTML = document.querySelector('#no-content').innerHTML;
        rawContent.textContent = '';
      } else {
        renderContent.innerHTML = data.content;
        rawContent.textContent = comboMarkdownEditor.value();
        const refIssues = renderContent.querySelectorAll('p .ref-issue');
        attachRefIssueContextPopup(refIssues);
      }
      const content = segment;
      if (!content.querySelector('.dropzone-attachments')) {
        if (data.attachments !== '') {
          content.insertAdjacentHTML('beforeend', data.attachments);
        }
      } else if (data.attachments === '') {
        content.querySelector('.dropzone-attachments').remove();
      } else {
        content.querySelector('.dropzone-attachments').outerHTML = data.attachments;
      }
      comboMarkdownEditor.dropzoneSubmitReload();
      initMarkupContent();
      initCommentContent();
    } catch (error) {
      showErrorToast(`Failed to save the content: ${error}`);
      console.error(error);
    } finally {
      renderContent.classList.remove('is-loading');
    }
  };

  // Show write/preview tab and copy raw content as needed
  showElem(editContentZone);
  hideElem(renderContent);

  comboMarkdownEditor = getComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
  if (!comboMarkdownEditor) {
    editContentZone.innerHTML = document.querySelector('#issue-comment-editor-template').innerHTML;
    const form = editContentZone.querySelector('form');
    applyAreYouSure(form);
    const saveButton = querySingleVisibleElem<HTMLButtonElement>(editContentZone, '.ui.primary.button');
    const cancelButton = querySingleVisibleElem<HTMLButtonElement>(editContentZone, '.ui.cancel.button');
    comboMarkdownEditor = await initComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
    const syncUiState = () => saveButton.disabled = comboMarkdownEditor.isUploading();
    comboMarkdownEditor.container.addEventListener(ComboMarkdownEditor.EventUploadStateChanged, syncUiState);
    cancelButton.addEventListener('click', cancelAndReset);
    form.addEventListener('submit', saveAndRefresh);
  }

  // FIXME: ideally here should reload content and attachment list from backend for existing editor, to avoid losing data
  if (!comboMarkdownEditor.value()) {
    comboMarkdownEditor.value(rawContent.textContent);
  }
  comboMarkdownEditor.switchTabToEditor();
  comboMarkdownEditor.focus();
  triggerUploadStateChanged(comboMarkdownEditor.container);
}

function extractSelectedMarkdown(container: HTMLElement) {
  const selection = window.getSelection();
  if (!selection.rangeCount) return '';
  const range = selection.getRangeAt(0);
  if (!container.contains(range.commonAncestorContainer)) return '';

  // todo: if commonAncestorContainer parent has "[data-markdown-original-content]" attribute, use the parent's markdown content
  // otherwise, use the selected HTML content and respect all "[data-markdown-original-content]/[data-markdown-generated-content]" attributes
  const contents = selection.getRangeAt(0).cloneContents();
  const el = document.createElement('div');
  el.append(contents);
  return convertHtmlToMarkdown(el);
}

async function tryOnQuoteReply(e) {
  const clickTarget = (e.target as HTMLElement).closest('.quote-reply');
  if (!clickTarget) return;

  e.preventDefault();
  const contentToQuoteId = clickTarget.getAttribute('data-target');
  const targetRawToQuote = document.querySelector<HTMLElement>(`#${contentToQuoteId}.raw-content`);
  const targetMarkupToQuote = targetRawToQuote.parentElement.querySelector<HTMLElement>('.render-content.markup');
  let contentToQuote = extractSelectedMarkdown(targetMarkupToQuote);
  if (!contentToQuote) contentToQuote = targetRawToQuote.textContent;
  const quotedContent = `${contentToQuote.replace(/^/mg, '> ')}\n`;

  let editor;
  if (clickTarget.classList.contains('quote-reply-diff')) {
    const replyBtn = clickTarget.closest('.comment-code-cloud').querySelector('button.comment-form-reply');
    editor = await handleReply(replyBtn);
  } else {
    // for normal issue/comment page
    editor = getComboMarkdownEditor(document.querySelector('#comment-form .combo-markdown-editor'));
  }

  if (editor.value()) {
    editor.value(`${editor.value()}\n\n${quotedContent}`);
  } else {
    editor.value(quotedContent);
  }
  editor.focus();
  editor.moveCursorToEnd();
}

export function initRepoIssueCommentEdit() {
  document.addEventListener('click', (e) => {
    tryOnEditContent(e); // Edit issue or comment content
    tryOnQuoteReply(e); // Quote reply to the comment editor
  });
}
