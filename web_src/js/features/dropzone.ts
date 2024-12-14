import {svg} from '../svg.ts';
import {htmlEscape} from 'escape-goat';
import {clippie} from 'clippie';
import {showTemporaryTooltip} from '../modules/tippy.ts';
import {GET, POST} from '../modules/fetch.ts';
import {showErrorToast} from '../modules/toast.ts';
import {createElementFromHTML, createElementFromAttrs} from '../utils/dom.ts';
import {isImageFile, isVideoFile} from '../utils.ts';
import type {DropzoneFile} from 'dropzone/index.js';

const {csrfToken, i18n} = window.config;

// dropzone has its owner event dispatcher (emitter)
export const DropzoneCustomEventReloadFiles = 'dropzone-custom-reload-files';
export const DropzoneCustomEventRemovedFile = 'dropzone-custom-removed-file';
export const DropzoneCustomEventUploadDone = 'dropzone-custom-upload-done';

async function createDropzone(el, opts) {
  const [{default: Dropzone}] = await Promise.all([
    import(/* webpackChunkName: "dropzone" */'dropzone'),
    import(/* webpackChunkName: "dropzone" */'dropzone/dist/dropzone.css'),
  ]);
  return new Dropzone(el, opts);
}

export function generateMarkdownLinkForAttachment(file, {width, dppx}: {width?: number, dppx?: number} = {}) {
  let fileMarkdown = `[${file.name}](/attachments/${file.uuid})`;
  if (isImageFile(file)) {
    fileMarkdown = `!${fileMarkdown}`;
    if (width > 0 && dppx > 1) {
      // Scale down images from HiDPI monitors. This uses the <img> tag because it's the only
      // method to change image size in Markdown that is supported by all implementations.
      // Make the image link relative to the repo path, then the final URL is "/sub-path/owner/repo/attachments/{uuid}"
      fileMarkdown = `<img width="${Math.round(width / dppx)}" alt="${htmlEscape(file.name)}" src="attachments/${htmlEscape(file.uuid)}">`;
    } else {
      // Markdown always renders the image with a relative path, so the final URL is "/sub-path/owner/repo/attachments/{uuid}"
      // TODO: it should also use relative path for consistency, because absolute is ambiguous for "/sub-path/attachments" or "/attachments"
      fileMarkdown = `![${file.name}](/attachments/${file.uuid})`;
    }
  } else if (isVideoFile(file)) {
    fileMarkdown = `<video src="attachments/${htmlEscape(file.uuid)}" title="${htmlEscape(file.name)}" controls></video>`;
  }
  return fileMarkdown;
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
    const success = await clippie(generateMarkdownLinkForAttachment(file));
    showTemporaryTooltip(e.target as Element, success ? i18n.copy_success : i18n.copy_error);
  });
  file.previewTemplate.append(copyLinkEl);
}

/**
 * @param {HTMLElement} dropzoneEl
 */
export async function initDropzone(dropzoneEl: HTMLElement) {
  const listAttachmentsUrl = dropzoneEl.closest('[data-attachment-url]')?.getAttribute('data-attachment-url');
  const removeAttachmentUrl = dropzoneEl.getAttribute('data-remove-url');
  const attachmentBaseLinkUrl = dropzoneEl.getAttribute('data-link-url');

  let disableRemovedfileEvent = false; // when resetting the dropzone (removeAllFiles), disable the "removedfile" event
  let fileUuidDict = {}; // to record: if a comment has been saved, then the uploaded files won't be deleted from server when clicking the Remove in the dropzone
  const opts: Record<string, any> = {
    url: dropzoneEl.getAttribute('data-upload-url'),
    headers: {'X-Csrf-Token': csrfToken},
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
  };
  if (dropzoneEl.hasAttribute('data-max-file')) opts.maxFiles = Number(dropzoneEl.getAttribute('data-max-file'));
  if (dropzoneEl.hasAttribute('data-max-size')) opts.maxFilesize = Number(dropzoneEl.getAttribute('data-max-size'));

  // there is a bug in dropzone: if a non-image file is uploaded, then it tries to request the file from server by something like:
  // "http://localhost:3000/owner/repo/issues/[object%20Event]"
  // the reason is that the preview "callback(dataURL)" is assign to "img.onerror" then "thumbnail" uses the error object as the dataURL and generates '<img src="[object Event]">'
  const dzInst = await createDropzone(dropzoneEl, opts);
  dzInst.on('success', (file: DropzoneFile & {uuid: string}, resp: any) => {
    file.uuid = resp.uuid;
    fileUuidDict[file.uuid] = {submitted: false};
    const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${resp.uuid}`, value: resp.uuid});
    dropzoneEl.querySelector('.files').append(input);
    addCopyLink(file);
    dzInst.emit(DropzoneCustomEventUploadDone, {file});
  });

  dzInst.on('removedfile', async (file: DropzoneFile & {uuid: string}) => {
    if (disableRemovedfileEvent) return;

    dzInst.emit(DropzoneCustomEventRemovedFile, {fileUuid: file.uuid});
    document.querySelector(`#dropzone-file-${file.uuid}`)?.remove();
    // when the uploaded file number reaches the limit, there is no uuid in the dict, and it doesn't need to be removed from server
    if (removeAttachmentUrl && fileUuidDict[file.uuid] && !fileUuidDict[file.uuid].submitted) {
      await POST(removeAttachmentUrl, {data: new URLSearchParams({file: file.uuid})});
    }
  });

  dzInst.on('submit', () => {
    for (const fileUuid of Object.keys(fileUuidDict)) {
      fileUuidDict[fileUuid].submitted = true;
    }
  });

  dzInst.on(DropzoneCustomEventReloadFiles, async () => {
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
        const file = {name: attachment.name, uuid: attachment.uuid, size: attachment.size};
        dzInst.emit('addedfile', file);
        dzInst.emit('complete', file);
        if (isImageFile(file.name)) {
          const imgSrc = `${attachmentBaseLinkUrl}/${file.uuid}`;
          dzInst.emit('thumbnail', file, imgSrc);
        }
        addCopyLink(file); // it is from server response, so no "type"
        fileUuidDict[file.uuid] = {submitted: true};
        const input = createElementFromAttrs('input', {name: 'files', type: 'hidden', id: `dropzone-file-${file.uuid}`, value: file.uuid});
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

  if (listAttachmentsUrl) dzInst.emit(DropzoneCustomEventReloadFiles);
  return dzInst;
}
