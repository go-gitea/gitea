import $ from 'jquery';
import {handleReply} from './repo-issue.ts';
import {getComboMarkdownEditor, initComboMarkdownEditor, ComboMarkdownEditor} from './comp/ComboMarkdownEditor.ts';
import {POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {hideElem, showElem} from '../utils/dom.ts';
import {attachRefIssueContextPopup} from './contextpopup.ts';
import {initCommentContent, initMarkupContent} from '../markup/content.ts';
import {triggerUploadStateChanged} from './comp/EditorUpload.ts';

async function onEditContent(event) {
  event.preventDefault();

  const segment = this.closest('.header').nextElementSibling;
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

  comboMarkdownEditor = getComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
  if (!comboMarkdownEditor) {
    editContentZone.innerHTML = document.querySelector('#issue-comment-editor-template').innerHTML;
    const saveButton = editContentZone.querySelector('.ui.primary.button');
    comboMarkdownEditor = await initComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
    const syncUiState = () => saveButton.disabled = comboMarkdownEditor.isUploading();
    comboMarkdownEditor.container.addEventListener(ComboMarkdownEditor.EventUploadStateChanged, syncUiState);
    editContentZone.querySelector('.ui.cancel.button').addEventListener('click', cancelAndReset);
    saveButton.addEventListener('click', saveAndRefresh);
  }

  // Show write/preview tab and copy raw content as needed
  showElem(editContentZone);
  hideElem(renderContent);
  // FIXME: ideally here should reload content and attachment list from backend for existing editor, to avoid losing data
  if (!comboMarkdownEditor.value()) {
    comboMarkdownEditor.value(rawContent.textContent);
  }
  comboMarkdownEditor.switchTabToEditor();
  comboMarkdownEditor.focus();
  triggerUploadStateChanged(comboMarkdownEditor.container);
}

export function initRepoIssueCommentEdit() {
  // Edit issue or comment content
  $(document).on('click', '.edit-content', onEditContent);

  // Quote reply
  $(document).on('click', '.quote-reply', async function (event) {
    event.preventDefault();
    const target = this.getAttribute('data-target');
    const quote = document.querySelector(`#${target}`).textContent.replace(/\n/g, '\n> ');
    const content = `> ${quote}\n\n`;

    let editor;
    if (this.classList.contains('quote-reply-diff')) {
      const replyBtn = this.closest('.comment-code-cloud').querySelector('button.comment-form-reply');
      editor = await handleReply(replyBtn);
    } else {
      // for normal issue/comment page
      editor = getComboMarkdownEditor($('#comment-form .combo-markdown-editor'));
    }
    if (editor) {
      if (editor.value()) {
        editor.value(`${editor.value()}\n\n${content}`);
      } else {
        editor.value(content);
      }
      editor.focus();
      editor.moveCursorToEnd();
    }
  });
}
