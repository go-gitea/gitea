import $ from 'jquery';
import {handleReply} from './repo-issue.js';
import {getComboMarkdownEditor, initComboMarkdownEditor} from './comp/ComboMarkdownEditor.js';
import {createDropzone} from './dropzone.js';
import {GET, POST} from '../modules/fetch.js';
import {hideElem, showElem} from '../utils/dom.js';
import {attachRefIssueContextPopup} from './contextpopup.js';
import {initCommentContent, initMarkupContent} from '../markup/content.js';

const {csrfToken} = window.config;

async function onEditContent(event) {
  event.preventDefault();

  const segment = this.closest('.header').nextElementSibling;
  const editContentZone = segment.querySelector('.edit-content-zone');
  const renderContent = segment.querySelector('.render-content');
  const rawContent = segment.querySelector('.raw-content');

  let comboMarkdownEditor;

  /**
   * @param {HTMLElement} dropzone
   */
  const setupDropzone = async (dropzone) => {
    if (!dropzone) return null;

    let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
    let fileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone
    const dz = await createDropzone(dropzone, {
      url: dropzone.getAttribute('data-upload-url'),
      headers: {'X-Csrf-Token': csrfToken},
      maxFiles: dropzone.getAttribute('data-max-file'),
      maxFilesize: dropzone.getAttribute('data-max-size'),
      acceptedFiles: ['*/*', ''].includes(dropzone.getAttribute('data-accepts')) ? null : dropzone.getAttribute('data-accepts'),
      addRemoveLinks: true,
      dictDefaultMessage: dropzone.getAttribute('data-default-message'),
      dictInvalidFileType: dropzone.getAttribute('data-invalid-input-type'),
      dictFileTooBig: dropzone.getAttribute('data-file-too-big'),
      dictRemoveFile: dropzone.getAttribute('data-remove-file'),
      timeout: 0,
      thumbnailMethod: 'contain',
      thumbnailWidth: 480,
      thumbnailHeight: 480,
      init() {
        this.on('success', (file, data) => {
          file.uuid = data.uuid;
          fileUuidDict[file.uuid] = {submitted: false};
          const input = document.createElement('input');
          input.id = data.uuid;
          input.name = 'files';
          input.type = 'hidden';
          input.value = data.uuid;
          dropzone.querySelector('.files').append(input);
        });
        this.on('removedfile', async (file) => {
          document.getElementById(file.uuid)?.remove();
          if (disableRemovedfileEvent) return;
          if (dropzone.getAttribute('data-remove-url') && !fileUuidDict[file.uuid].submitted) {
            try {
              await POST(dropzone.getAttribute('data-remove-url'), {data: new URLSearchParams({file: file.uuid})});
            } catch (error) {
              console.error(error);
            }
          }
        });
        this.on('submit', () => {
          for (const fileUuid of Object.keys(fileUuidDict)) {
            fileUuidDict[fileUuid].submitted = true;
          }
        });
        this.on('reload', async () => {
          try {
            const response = await GET(editContentZone.getAttribute('data-attachment-url'));
            const data = await response.json();
            // do not trigger the "removedfile" event, otherwise the attachments would be deleted from server
            disableRemovedfileEvent = true;
            dz.removeAllFiles(true);
            dropzone.querySelector('.files').innerHTML = '';
            for (const el of dropzone.querySelectorAll('.dz-preview')) el.remove();
            fileUuidDict = {};
            disableRemovedfileEvent = false;

            for (const attachment of data) {
              const imgSrc = `${dropzone.getAttribute('data-link-url')}/${attachment.uuid}`;
              dz.emit('addedfile', attachment);
              dz.emit('thumbnail', attachment, imgSrc);
              dz.emit('complete', attachment);
              fileUuidDict[attachment.uuid] = {submitted: true};
              dropzone.querySelector(`img[src='${imgSrc}']`).style.maxWidth = '100%';
              const input = document.createElement('input');
              input.id = attachment.uuid;
              input.name = 'files';
              input.type = 'hidden';
              input.value = attachment.uuid;
              dropzone.querySelector('.files').append(input);
            }
            if (!dropzone.querySelector('.dz-preview')) {
              dropzone.classList.remove('dz-started');
            }
          } catch (error) {
            console.error(error);
          }
        });
      },
    });
    dz.emit('reload');
    return dz;
  };

  const cancelAndReset = (e) => {
    e.preventDefault();
    showElem(renderContent);
    hideElem(editContentZone);
    comboMarkdownEditor.attachedDropzoneInst?.emit('reload');
  };

  const saveAndRefresh = async (e) => {
    e.preventDefault();
    showElem(renderContent);
    hideElem(editContentZone);
    const dropzoneInst = comboMarkdownEditor.attachedDropzoneInst;
    try {
      const params = new URLSearchParams({
        content: comboMarkdownEditor.value(),
        context: editContentZone.getAttribute('data-context'),
      });
      for (const fileInput of dropzoneInst?.element.querySelectorAll('.files [name=files]')) params.append('files[]', fileInput.value);

      const response = await POST(editContentZone.getAttribute('data-update-url'), {data: params});
      const data = await response.json();
      if (!data.content) {
        renderContent.innerHTML = document.getElementById('no-content').innerHTML;
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
      dropzoneInst?.emit('submit');
      dropzoneInst?.emit('reload');
      initMarkupContent();
      initCommentContent();
    } catch (error) {
      console.error(error);
    }
  };

  comboMarkdownEditor = getComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
  if (!comboMarkdownEditor) {
    editContentZone.innerHTML = document.getElementById('issue-comment-editor-template').innerHTML;
    comboMarkdownEditor = await initComboMarkdownEditor(editContentZone.querySelector('.combo-markdown-editor'));
    comboMarkdownEditor.attachedDropzoneInst = await setupDropzone(editContentZone.querySelector('.dropzone'));
    editContentZone.querySelector('.ui.cancel.button').addEventListener('click', cancelAndReset);
    editContentZone.querySelector('.ui.primary.button').addEventListener('click', saveAndRefresh);
  }

  // Show write/preview tab and copy raw content as needed
  showElem(editContentZone);
  hideElem(renderContent);
  if (!comboMarkdownEditor.value()) {
    comboMarkdownEditor.value(rawContent.textContent);
  }
  comboMarkdownEditor.focus();
}

export function initRepoIssueCommentEdit() {
  // Edit issue or comment content
  $(document).on('click', '.edit-content', onEditContent);

  // Quote reply
  $(document).on('click', '.quote-reply', async function (event) {
    event.preventDefault();
    const target = $(this).data('target');
    const quote = $(`#${target}`).text().replace(/\n/g, '\n> ');
    const content = `> ${quote}\n\n`;
    let editor;
    if ($(this).hasClass('quote-reply-diff')) {
      const $replyBtn = $(this).closest('.comment-code-cloud').find('button.comment-form-reply');
      editor = await handleReply($replyBtn);
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
