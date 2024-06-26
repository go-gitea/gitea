import {svg} from '../svg.js';
import {htmlEscape} from 'escape-goat';
import {clippie} from 'clippie';
import {showTemporaryTooltip} from '../modules/tippy.js';
import {GET, POST} from '../modules/fetch.js';
import {showErrorToast} from '../modules/toast.js';
import {createElementFromHTML, createElementFromAttrs, elemGetAttributeNumber} from '../utils/dom.js';

const {csrfToken, i18n} = window.config;

async function createDropzone(el, opts) {
  const [{Dropzone}] = await Promise.all([
    import(/* webpackChunkName: "dropzone" */'dropzone'),
    import(/* webpackChunkName: "dropzone" */'dropzone/dist/dropzone.css'),
  ]);
  return new Dropzone(el, opts);
}

function addCopyLink(file) {
  // Create a "Copy Link" element, to conveniently copy the image or file link as Markdown to the clipboard
  // The "<a>" element has a hardcoded cursor: pointer because the default is overridden by .dropzone
  const copyLinkEl = createElementFromHTML(`
<div class="tw-text-center">
  <a href="#" class="tw-cursor-pointer">${svg('octicon-copy', 14)} Copy link</a>
</div>`);
  copyLinkEl.addEventListener('click', async (e) => {
    e.preventDefault();
    let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
    if (file.type?.startsWith('image/')) {
      fileMarkdown = `!${fileMarkdown}`;
    } else if (file.type?.startsWith('video/')) {
      fileMarkdown = `<video src="/attachments/${htmlEscape(file.uuid)}" title="${htmlEscape(file.name)}" controls></video>`;
    }
    const success = await clippie(fileMarkdown);
    showTemporaryTooltip(e.target, success ? i18n.copy_success : i18n.copy_error);
  });
  file.previewTemplate.append(copyLinkEl);
}

/**
 * @param {HTMLElement} dropzoneEl
 */
export async function initDropzone(dropzoneEl) {
  if (!dropzoneEl) return null;

  const listAttachmentsUrl = dropzoneEl.closest('[data-attachment-url]')?.getAttribute('data-attachment-url');
  const removeAttachmentUrl = dropzoneEl.getAttribute('data-remove-url');
  const attachmentBaseLinkUrl = dropzoneEl.getAttribute('data-link-url');

  let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
  let fileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone
  const dzInst = await createDropzone(dropzoneEl, {
    url: dropzoneEl.getAttribute('data-upload-url'),
    headers: {'X-Csrf-Token': csrfToken},
    maxFiles: elemGetAttributeNumber('data-max-file', null), // match dropzone default value, no limit
    maxFilesize: elemGetAttributeNumber('data-max-size', 256), // match dropzone default value: 256 MiB
    acceptedFiles: ['*/*', ''].includes(dropzoneEl.getAttribute('data-accepts')) ? null : dropzoneEl.getAttribute('data-accepts'),
    addRemoveLinks: true,
    dictDefaultMessage: dropzoneEl.getAttribute('data-default-message'),
    dictInvalidFileType: dropzoneEl.getAttribute('data-invalid-input-type'),
    dictFileTooBig: dropzoneEl.getAttribute('data-file-too-big'),
    dictRemoveFile: dropzoneEl.getAttribute('data-remove-file'),
    timeout: 0,
    thumbnailMethod: 'contain',
    thumbnailWidth: 480,
    thumbnailHeight: 480,
  });

  dzInst.on('success', (file, data) => {
    file.uuid = data.uuid;
    fileUuidDict[file.uuid] = {submitted: false};
    const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${data.uuid}`, value: data.uuid});
    dropzoneEl.querySelector('.files').append(input);
    addCopyLink(file);
  });

  dzInst.on('removedfile', async (file) => {
    if (disableRemovedfileEvent) return;
    document.querySelector(`#dropzone-file-${file.uuid}`)?.remove();
    if (removeAttachmentUrl && !fileUuidDict[file.uuid].submitted) {
      await POST(removeAttachmentUrl, {data: new URLSearchParams({file: file.uuid})});
    }
  });

  dzInst.on('submit', () => {
    for (const fileUuid of Object.keys(fileUuidDict)) {
      fileUuidDict[fileUuid].submitted = true;
    }
  });

  dzInst.on('reload', async () => {
    try {
      const resp = await GET(listAttachmentsUrl);
      const respData = await resp.json();
      // do not trigger the "removedfile" event, otherwise the attachments would be deleted from server
      disableRemovedfileEvent = true;
      dzInst.removeAllFiles(true);
      disableRemovedfileEvent = false;

      dropzoneEl.querySelector('.files').innerHTML = '';
      for (const el of dropzoneEl.querySelectorAll('.dz-preview')) el.remove();
      fileUuidDict = {};
      for (const attachment of respData) {
        const imgSrc = `${attachmentBaseLinkUrl}/${attachment.uuid}`;
        dzInst.emit('addedfile', attachment);
        dzInst.emit('thumbnail', attachment, imgSrc);
        dzInst.emit('complete', attachment);
        addCopyLink(attachment);
        fileUuidDict[attachment.uuid] = {submitted: true};
        const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${attachment.uuid}`, value: attachment.uuid});
        dropzoneEl.querySelector('.files').append(input);
      }
      if (!dropzoneEl.querySelector('.dz-preview')) {
        dropzoneEl.classList.remove('dz-started');
      }
    } catch (error) {
      // TODO: if listing the existing attachments failed, it should stop from operating the content or attachments,
      //  otherwise the attachments might be lost.
      showErrorToast(`Failed to load attachments: ${error}`);
      console.error(error);
    }
  });

  dzInst.on('error', (file, message) => {
    showErrorToast(`Dropzone upload error: ${message}`);
    dzInst.removeFile(file);
  });

  if (listAttachmentsUrl) dzInst.emit('reload');
  return dzInst;
}
